#!/bin/sh
# Runs the Playwright suite inside a CPU-limited container so the browsers do not saturate the host.
# Usage: scripts/e2e-docker.sh [playwright test arguments], e.g. scripts/e2e-docker.sh --project=desktop e2e/specs/auth
# The app under test is the dev stack: the container joins its compose network and forwards localhost:5173 to the ui service and localhost:8080 to the api service (secure context, so the clipboard works).
# The e2e password comes from the git-ignored web/.env.e2e (or E2E_ENV_FILE) passed with --env-file, never echoed.
# Limits (override with env): DOCKER_CPUS (default 4), DOCKER_MEMORY (default 6g), PW_WORKERS (default 2),
# PW_OUT (host dir receiving test-results; default web/test-results), PW_IMAGE (default mcr.microsoft.com/playwright:v1.65.1-noble), E2E_BASE_URL, E2E_API_URL.
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd -W 2>/dev/null || pwd)"
ENV_FILE="${E2E_ENV_FILE:-$ROOT/.env.e2e}"
exec docker run --rm --ipc=host \
  --cpus="${DOCKER_CPUS:-4}" --memory="${DOCKER_MEMORY:-6g}" \
  --env-file "$ENV_FILE" \
  -e CI= -e E2E_ENV_FILE= -e PW_WORKERS="${PW_WORKERS:-2}" \
  -e E2E_BASE_URL="${E2E_BASE_URL:-http://localhost:5173}" \
  -e E2E_API_URL="${E2E_API_URL:-http://localhost:8080/api/v1}" \
  --network "${E2E_NETWORK:-corelog-local_default}" \
  -v "$ROOT:/app" \
  -v corelog-local_web_dev_node_modules:/app/node_modules:ro \
  -v "${PW_OUT:-$ROOT/test-results}:/app/test-results" -v /app/playwright-report \
  -w /app \
  "${PW_IMAGE:-mcr.microsoft.com/playwright:v1.65.1-noble}" \
  sh -c 'node scripts/e2e-forward.cjs ui:4300=5173 api:8080=8080 & exec npx playwright test --workers="$PW_WORKERS" --reporter=list "$@"' sh "$@"
