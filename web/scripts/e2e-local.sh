#!/bin/sh
# Runs the Playwright suite natively against the host dev server (scripts/dev-host.sh on :5173) and the API
# on :8080. Only ONE run at a time (shared stack and database): the script takes the shared lock and waits.
# Usage: scripts/e2e-local.sh [playwright test arguments], e.g. scripts/e2e-local.sh --project=desktop e2e/specs/auth
# Workers default to 8 (PW_WORKERS). The e2e password comes from the git-ignored web/.env.e2e, never echoed.
# Visual snapshot tests stay off (their baselines are Linux-only: regenerate them with scripts/e2e-docker.sh).
set -eu
. "$(dirname "$0")/docker-lock.sh"
acquire_docker_lock
cd "$(dirname "$0")/.."
E2E_BASE_URL="${E2E_BASE_URL:-http://localhost:5173}" E2E_API_URL="${E2E_API_URL:-http://localhost:8080/api/v1}" \
  npx playwright test --workers="${PW_WORKERS:-8}" --reporter=list "$@"
