SHELL := /bin/zsh
.DEFAULT_GOAL := help

ifneq (,$(wildcard .env))
include .env
export
endif

DATABASE_URL ?= postgres://opora:opora_local_postgres_change_me@localhost:5432/opora?sslmode=disable

.PHONY: help bootstrap up down logs api web dev migrate generate test test-backend test-frontend lint lint-backend lint-frontend health compose-config

help:
	@echo "make bootstrap       Verify the macOS development toolchain"
	@echo "make up              Start PostgreSQL, MinIO, ONLYOFFICE and ClamAV"
	@echo "make down            Stop local infrastructure"
	@echo "make migrate         Apply PostgreSQL migrations"
	@echo "make api             Run the Go API"
	@echo "make web             Run the Next.js application"
	@echo "make dev             Start infrastructure, API and frontend"
	@echo "make generate        Run sqlc and OpenAPI type generation"
	@echo "make test            Run backend and frontend tests"
	@echo "make lint            Run backend and frontend checks"

bootstrap:
	./scripts/bootstrap-macos.sh

up:
	docker compose up -d
	docker compose up -d --wait postgres minio onlyoffice clamav

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

api:
	cd apps/api && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

web:
	pnpm --dir apps/web dev

dev:
	./scripts/dev.sh

migrate:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir db/migrations postgres "$(DATABASE_URL)" up

generate:
	cd apps/api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
	pnpm --dir apps/web generate:api

test: test-backend test-frontend

test-backend:
	cd apps/api && go test -race ./...

test-frontend:
	pnpm --dir apps/web test

lint: lint-backend lint-frontend

lint-backend:
	cd apps/api && gofmt -w .
	cd apps/api && go vet ./...
	cd apps/api && golangci-lint run ./...
	cd apps/api && go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

lint-frontend:
	pnpm --dir apps/web lint
	pnpm --dir apps/web typecheck

health:
	curl --fail --silent --show-error http://localhost:8080/health/live
	curl --fail --silent --show-error http://localhost:8080/health/ready

compose-config:
	docker compose config --quiet
