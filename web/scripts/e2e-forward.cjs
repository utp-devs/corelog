// Forwards localhost ports of the Playwright container to the host (the dev stack) so the app is
// opened on http://localhost:<port>: a secure context, exactly like a developer's browser, which the
// clipboard API requires. Usage: node scripts/e2e-forward.cjs host.docker.internal 5173 8080
const net = require('node:net');

const [host, ...ports] = process.argv.slice(2);
for (const port of ports.map(Number)) {
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
    .listen(port, '127.0.0.1');
}
