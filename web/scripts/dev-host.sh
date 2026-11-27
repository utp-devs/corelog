#!/bin/sh
# Native (Windows/Linux/macOS) development server: builds the three apps once, serves them with the API
# proxy on http://localhost:5173 (portal /, workspace /workspace/, admin /admin/) and rebuilds on change.
# The API and the database keep running in Docker (docker-compose.local.yaml). No polling is needed natively.
# Usage: scripts/dev-host.sh            (watch every app)
#        WATCH=workspace scripts/dev-host.sh
cd "$(dirname "$0")/.."
PORT="${PORT:-5173}" API_UPSTREAM="${API_UPSTREAM:-http://localhost:8080}" WATCH="${WATCH:-all}" exec node scripts/dev.mjs
