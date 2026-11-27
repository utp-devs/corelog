#!/bin/sh
# Runs a command (build, test, lint, format) inside a CPU-limited container so Node does not
# saturate the host. Usage: scripts/in-docker.sh "npx ng test core --watch=false"
# The container sees the sources through a bind mount, uses the Linux dependencies stored in the
# dev volume, and keeps its own throwaway dist and cache so parallel runs never collide.
# Limits (override with env): DOCKER_CPUS (default 4) and DOCKER_MEMORY (default 6g).
set -eu
if [ "$#" -lt 1 ]; then
  echo "usage: scripts/in-docker.sh \"<command>\"" >&2
  exit 2
fi
ROOT="$(cd "$(dirname "$0")/.." && pwd -W 2>/dev/null || pwd)"
exec docker run --rm \
  --cpus="${DOCKER_CPUS:-4}" --memory="${DOCKER_MEMORY:-6g}" \
  -e CI=true -e NG_CLI_ANALYTICS=false -e NODE_OPTIONS="--max-old-space-size=4096" \
  -v "$ROOT:/app" \
  -v corelog-local_web_dev_node_modules:/app/node_modules:ro \
  -v /app/dist -v /app/.angular -v /app/out-tsc \
  -w /app \
  corelog-local-ui:latest sh -c "$1"
