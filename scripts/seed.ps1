# Seeds the CoreLog database through cmd/seed.
#
#   $env:SEED_PASSWORD = "..."; .\scripts\seed.ps1 --scenario demo
#
# SEED_PASSWORD is required and is never printed. All other arguments are
# forwarded to cmd/seed unchanged.
$ErrorActionPreference = "Stop"

if (-not $env:SEED_PASSWORD) {
    Write-Host "SEED_PASSWORD es obligatorio: define la contrasena de las cuentas de prueba." -ForegroundColor Red
    exit 1
}

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    Write-Host "==> Sembrando la base de datos de CoreLog..." -ForegroundColor Cyan
    & go run ./cmd/seed @args
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}
