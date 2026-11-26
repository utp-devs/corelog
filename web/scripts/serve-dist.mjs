// Local static server that mirrors the nginx layout: portal at /, workspace at /workspace/,
// admin at /admin/, and /api proxied to API_UPSTREAM when it is set.
import { createReadStream, existsSync, statSync } from 'node:fs';
import http from 'node:http';
import { extname, join, normalize, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)), 'dist');
const port = Number(process.env.PORT ?? 4300);
const apiUpstream = process.env.API_UPSTREAM ? new URL(process.env.API_UPSTREAM) : null;

const APPS = [
  { name: 'portal', mount: '' },
  { name: 'workspace', mount: '/workspace' },
  { name: 'admin', mount: '/admin' },
];

const mimeTypes = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.map': 'application/json',
  '.txt': 'text/plain; charset=utf-8',
};

function appDir(app) {
  return join(root, app.name, 'browser');
}

function builtApps() {
  return APPS.filter((app) => existsSync(join(appDir(app), 'index.html')));
}

function safeFile(dir, relative) {
  const target = normalize(join(dir, relative));
  if (target !== dir && !target.startsWith(dir + sep)) {
    return null;
  }
  try {
    return statSync(target).isFile() ? target : null;
  } catch {
    return null;
  }
}

function send(res, file, noCache) {
  const type = mimeTypes[extname(file).toLowerCase()] ?? 'application/octet-stream';
  res.writeHead(200, {
    'Content-Type': type,
    'Cache-Control': noCache ? 'no-cache' : 'public, max-age=31536000, immutable',
  });
  createReadStream(file).pipe(res);
}

function serveApp(res, app, relative) {
  const dir = appDir(app);
  const exact = relative === '' || relative.endsWith('/') ? null : safeFile(dir, relative);
  if (exact) {
    send(res, exact, exact.endsWith('.html'));
    return;
  }
  if (extname(relative) && extname(relative) !== '.html') {
    res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    res.end('No encontrado');
    return;
  }
  send(res, join(dir, 'index.html'), true);
}

function proxy(req, res) {
  if (!apiUpstream) {
    res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    res.end('Sin proxy de API: defina API_UPSTREAM');
    return;
  }
  const headers = { ...req.headers, host: apiUpstream.host };
  delete headers.origin;
  delete headers.referer;
  const upstream = http.request(
    {
      hostname: apiUpstream.hostname,
      port: apiUpstream.port || 80,
      path: req.url,
      method: req.method,
      headers,
    },
    (response) => {
      res.writeHead(response.statusCode ?? 502, response.headers);
      response.pipe(res);
    },
  );
  upstream.on('error', () => {
    if (!res.headersSent) {
      res.writeHead(502, { 'Content-Type': 'text/plain; charset=utf-8' });
    }
    res.end('API no disponible');
  });
  res.on('close', () => upstream.destroy());
  req.pipe(upstream);
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url ?? '/', 'http://localhost');
  const path = decodeURIComponent(url.pathname);
  if (path === '/healthz') {
    res.writeHead(200, { 'Content-Type': 'text/plain; charset=utf-8' });
    res.end('ok');
    return;
  }
  if (path.startsWith('/api/') || path === '/api') {
    proxy(req, res);
    return;
  }
  const apps = builtApps();
  const mounted = apps.find(
    (app) => app.mount && (path === app.mount || path.startsWith(app.mount + '/')),
  );
  if (mounted) {
    if (path === mounted.mount) {
      res.writeHead(301, { Location: `${mounted.mount}/${url.search}` });
      res.end();
      return;
    }
    serveApp(res, mounted, path.slice(mounted.mount.length + 1));
    return;
  }
  const portal = apps.find((app) => app.name === 'portal');
  if (!portal) {
    res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    res.end('Falta dist/portal');
    return;
  }
  serveApp(res, portal, path.replace(/^\/+/, ''));
});

server.requestTimeout = 0;
server.listen(port, () => {
  const names =
    builtApps()
      .map((app) => app.name)
      .join(', ') || 'sin aplicaciones todavía';
  console.log(`CoreLog sirviendo ${names} en http://localhost:${port}`);
});
