#!/usr/bin/env bash
set -euo pipefail

echo "==> Running CoreLog database seed..."
go run ./cmd/seed
