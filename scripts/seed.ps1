$ErrorActionPreference = "Stop"

Write-Host "==> Running CoreLog database seed..." -ForegroundColor Cyan
go run ./cmd/seed
