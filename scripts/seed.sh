#!/usr/bin/env bash
# Seeds the CoreLog database through cmd/seed.
#
#   SEED_PASSWORD=... ./scripts/seed.sh [--scenario demo|e2e|scale] [--issues N] [--force-reset]
#
# SEED_PASSWORD is required and is never printed. All other arguments are
# forwarded to cmd/seed unchanged.
set -euo pipefail

if [ -z "${SEED_PASSWORD:-}" ]; then
  echo "SEED_PASSWORD es obligatorio: define la contrasena de las cuentas de prueba." >&2
  exit 1
fi

cd "$(dirname "$0")/.."
echo "==> Sembrando la base de datos de CoreLog..."
go run ./cmd/seed "$@"
