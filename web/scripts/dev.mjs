// Development entry for the Docker dev image: builds the three apps once, serves them through
// serve-dist and rebuilds the apps listed in WATCH (comma separated, or "all") on change.
import { spawn } from 'node:child_process';

const apps = ['portal', 'workspace', 'admin'];
const watched = (process.env.WATCH ?? '')
  .split(',')
  .map((name) => name.trim())
  .filter(Boolean);
const selected = watched.includes('all') ? apps : apps.filter((app) => watched.includes(app));
const poll = process.env.POLL ?? '1000';

const children = [];

function run(name, args, waitForEnd = false) {
  return new Promise((resolve) => {
    const child = spawn('npx', args, { stdio: ['ignore', 'pipe', 'pipe'], shell: false });
    children.push(child);
    let settled = false;
    const listen = (stream, target) => {
      stream.on('data', (chunk) => {
        const text = chunk.toString();
        target.write(text.replace(/^/gm, `[${name}] `));
        if (!waitForEnd && !settled && /bundle generation complete|ERROR/i.test(text)) {
          settled = true;
          resolve();
        }
      });
    };
    listen(child.stdout, process.stdout);
    listen(child.stderr, process.stderr);
    child.on('exit', (code) => {
      if (!settled) {
        settled = true;
        resolve();
      }
      console.log(`[${name}] terminó con código ${code}`);
    });
  });
}

function shutdown() {
  for (const child of children) {
    child.kill('SIGTERM');
  }
  process.exit(0);
}
process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

run('gateway', ['node', 'scripts/serve-dist.mjs']);

for (const app of apps.filter((name) => !selected.includes(name))) {
  await run(app, ['ng', 'build', app, '--configuration', 'local'], true);
}

for (const app of selected) {
  await run(app, ['ng', 'build', app, '--configuration', 'local', '--watch', '--poll', poll]);
}

console.log(
  selected.length
    ? `Aplicaciones compiladas. En recarga: ${selected.join(', ')}`
    : 'Aplicaciones compiladas. Sin recarga: define WATCH para vigilar alguna.',
);
