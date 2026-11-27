#!/bin/sh
# Shared lock so only ONE Node or Playwright container runs at a time (CPU limit and exclusive use of
# the dev stack). Source it, then call acquire_docker_lock; it releases on exit.
LOCK_DIR="${TMPDIR:-/tmp}/corelog-docker.lock"
acquire_docker_lock() {
  waited=0
  while ! mkdir "$LOCK_DIR" 2>/dev/null; do
    # A lock older than 90 minutes is stale (its owner died without cleaning up).
    if [ -n "$(find "$LOCK_DIR" -maxdepth 0 -mmin +90 2>/dev/null)" ]; then
      rmdir "$LOCK_DIR" 2>/dev/null || true
      continue
    fi
    waited=$((waited + 5))
    if [ "$waited" -ge 3600 ]; then
      echo "docker-lock: waited too long for $LOCK_DIR" >&2
      exit 3
    fi
    sleep 5
  done
  trap 'rmdir "$LOCK_DIR" 2>/dev/null || true' EXIT INT TERM
}
