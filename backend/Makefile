# Loads DB credentials from .env so migrate targets need no inline secrets.
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: run build test lint tidy docker-up docker-down migrate-up migrate-down migrate-new

run: ## Run the server on the host
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -race -cover

lint:
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| { echo "golangci-lint not installed — falling back to go vet"; go vet ./...; }

tidy:
	go mod tidy

docker-up: ## Start Postgres (add --profile full for the app container)
	docker compose up -d db

docker-down:
	docker compose down

migrate-up:
	go run ./cmd/migrate -dir up

migrate-down:
	go run ./cmd/migrate -dir down -steps 1

migrate-new: ## usage: make migrate-new name=add_users
	@n=$$(ls migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' '); \
	seq=$$(printf '%04d' $$((n + 1))); \
	touch migrations/$${seq}_$(name).up.sql migrations/$${seq}_$(name).down.sql; \
	echo "created migrations/$${seq}_$(name).{up,down}.sql"
