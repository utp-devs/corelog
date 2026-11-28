.PHONY: run build test test-race test-coverage lint tidy seed e2e e2e-visual stack-up stack-down \
	docker-up docker-down docker-logs \
	migrate-up migrate-down migrate-create \
	authz-audit perf-check \
	web-install web-format web-build web-test web-serve e2e-install e2e-native \
	ui-install ui-dev ui-build ui-lint ui-test

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./cmd/... ./internal/...

# The race detector needs cgo and a C compiler (gcc); CI runs it on ubuntu-latest.
test-race:
	go test -race -p 2 ./cmd/... ./internal/...

test-coverage:
	go test -coverprofile=coverage.out ./cmd/... ./internal/...
	go run ./cmd/coverage-check -min 70

# Authorization route walk and the 20,000 issue performance dataset (both need CORELOG_TEST_DATABASE_URL).
authz-audit:
	go test ./cmd/api -run "TestAuthz|TestPlatformAdministrator" -count=1 -v

perf-check:
	go test ./cmd/api -run TestPerformanceHotEndpoints -count=1 -v

seed:
	go run ./cmd/seed

e2e:
	cd web && npm run e2e

stack-up:
	docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml up -d --build

stack-down:
	docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml down

lint:
	go vet ./cmd/... ./internal/...

tidy:
	go mod tidy

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

DB_NAME ?= corelog
DB_URL ?= postgres://corelog:corelog@localhost:5432/$(DB_NAME)?sslmode=disable

migrate-up:
	migrate -path migrations/postgres -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations/postgres -database "$(DB_URL)" down 1

migrate-create:
	migrate create -ext sql -dir migrations/postgres -seq $(name)

# Web workflow, native (no Docker): Node 24 on the host.
WEB_PORT ?= 5173
API_URL ?= http://localhost:8080

web-install:
	cd web && npm ci --ignore-scripts

web-format:
	cd web && npm run format:check

web-build:
	cd web && npm run build:libs && npm run build:all:local

web-test:
	cd web && npm run test:all

# Serves web/dist like nginx does (portal at /, workspace at /workspace/, admin at /admin/) and proxies /api.
web-serve:
	cd web && PORT=$(WEB_PORT) API_UPSTREAM=$(API_URL) node scripts/serve-dist.mjs

e2e-install:
	cd web && npx playwright install --with-deps chromium

# Needs the API up (docker compose -f docker-compose.local.yaml up -d db migrate api), the seeded e2e scenario
# and make web-serve running.
e2e-native:
	cd web && E2E_BASE_URL=http://localhost:$(WEB_PORT) npx playwright test

e2e-visual:
	cd web && E2E_VISUAL=1 npm run e2e:visual

# Aliases kept for existing habits.
ui-install: web-install

ui-dev:
	cd web && npm run start:portal

ui-build: web-build

ui-lint: web-format

ui-test: web-test
