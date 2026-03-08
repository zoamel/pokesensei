-include .env
export

.PHONY: tools generate templ sqlc dev migrate setup import build clean

## Install development tools
tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/air-verse/air@latest

## Run all code generators
generate: templ sqlc

## Generate templ Go code from .templ files
templ:
	templ generate

## Generate sqlc Go code from SQL queries
sqlc:
	sqlc generate

## Start dev server with hot reload (requires Docker Compose running)
dev:
	air

## Run database migrations
migrate:
	goose -dir db/migrations postgres "$(DATABASE_URL)" up

## One-time setup: start Postgres, run migrations, import data if needed
setup:
	docker compose up -d
	@echo "Waiting for Postgres..."
	@if command -v pg_isready >/dev/null 2>&1; then \
		until pg_isready -h localhost -p 5432 -q 2>/dev/null; do sleep 0.5; done; \
	else \
		echo "(pg_isready not found, falling back to sleep)"; \
		sleep 3; \
	fi
	goose -dir db/migrations postgres "$(DATABASE_URL)" up
	@if [ "$$(psql "$(DATABASE_URL)" -tAc "SELECT count(*) FROM pokemon" 2>/dev/null)" = "0" ] || \
	    [ "$$(psql "$(DATABASE_URL)" -tAc "SELECT count(*) FROM pokemon" 2>/dev/null)" = "" ]; then \
		echo "Database empty — importing data..."; \
		go run ./cmd/import/ --database-url "$(DATABASE_URL)" --games frlg,hgss --seed-trainers; \
	else \
		echo "Data already imported — skipping (use 'make import' to force re-import)"; \
	fi

## Import PokéAPI data + trainer seeds (re-runnable, truncates and reloads)
import:
	go run ./cmd/import/ --database-url "$(DATABASE_URL)" --games frlg,hgss --seed-trainers

## Build production binary
build: generate
	go build -o bin/server ./cmd/server

## Remove build artifacts
clean:
	rm -rf bin/ tmp/
