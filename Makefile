SHELL := /bin/bash

DATABASE_URL ?= postgres://flashsale:flashsale@localhost:5432/flashsale?sslmode=disable
HTTP_ADDR    ?= :8080

.PHONY: up down logs migrate-up migrate-down migrate-reset \
        seed seed-fresh \
        run-backend run-frontend \
        test test-race test-integration loadtest \
        lint lint-backend lint-frontend

up:
	docker compose up -d postgres

down:
	docker compose down

logs:
	docker compose logs -f postgres

migrate-up:
	cd backend && go run ./cmd/server -migrate-only

migrate-down:
	cd backend && go run ./cmd/server -migrate-only -migrate-direction=down

migrate-reset:
	docker compose down -v && docker compose up -d postgres
	@echo "Waiting for postgres to be ready..."
	@until docker exec flashsale-postgres pg_isready -U flashsale >/dev/null 2>&1; do sleep 1; done
	$(MAKE) migrate-up

seed:
	cd backend && DATABASE_URL=$(DATABASE_URL) go run ./cmd/seed

seed-fresh:
	cd backend && DATABASE_URL=$(DATABASE_URL) go run ./cmd/seed -capacity 50

run-backend:
	cd backend && DATABASE_URL=$(DATABASE_URL) HTTP_ADDR=$(HTTP_ADDR) go run ./cmd/server

run-frontend:
	cd frontend && npm run dev

test:
	cd backend && go test ./internal/... ./loadtest/...

test-race:
	cd backend && go test -race ./...

test-integration:
	cd backend && DATABASE_URL=$(DATABASE_URL) go test -tags=integration ./tests/integration/...

loadtest:
	cd backend && go run ./loadtest \
		--target http://localhost:8080 \
		--capacity 10 \
		--concurrency 100 \
		--per-request-quantity 1

lint: lint-backend lint-frontend

lint-backend:
	cd backend && go vet ./... && (command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed; skipping")

lint-frontend:
	cd frontend && npm run lint
