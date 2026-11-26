.PHONY: run build test test-coverage lint tidy seed e2e stack-up stack-down \
	docker-up docker-down docker-logs \
	migrate-up migrate-down migrate-create \
	ui-install ui-dev ui-build ui-lint ui-test

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./cmd/... ./internal/...

test-coverage:
	go test -coverprofile=coverage.out ./cmd/... ./internal/...
	go run ./cmd/coverage-check -min 40

seed:
	go run ./cmd/seed

e2e:
	cd ui && npm run e2e

stack-up:
	docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml up -d --build

stack-down:
	docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml down

lint:
	go vet ./...

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

ui-install:
	cd ui && npm install

ui-dev:
	cd ui && npm run dev

ui-build:
	cd ui && npm run build

ui-lint:
	cd ui && npm run lint

ui-test:
	cd ui && npm test
