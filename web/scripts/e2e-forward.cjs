// Forwards localhost ports of the Playwright container to the host (the dev stack) so the app is
// opened on http://localhost:<port>: a secure context, exactly like a developer's browser, which the
// clipboard API requires. Each argument is <upstream host>:<upstream port>=<local port>, for example
// node scripts/e2e-forward.cjs ui:4300=5173 api:8080=8080
const net = require('node:net');

for (const spec of process.argv.slice(2)) {
  const [target, local] = spec.split('=');
  const [host, upstreamPort] = target.split(':');
  const port = Number(upstreamPort);
  const localPort = Number(local);
  net
    .createServer((client) => {
      const upstream = net.connect({ port, host });
      client.setNoDelay(true);
      upstream.setNoDelay(true);
      client.pipe(upstream);
      upstream.pipe(client);
      // Either side closing, failing or resetting tears down both, so no half-open socket lingers.
      const close = () => {
        client.destroy();
        upstream.destroy();
      };
      client.on('error', close);
      upstream.on('error', close);
      client.on('close', close);
      upstream.on('close', close);
    })
    .listen(localPort, '127.0.0.1');
}
