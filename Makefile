-include .env
export

DATABASE_PATH ?= data/pokesensei.db

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

## Start dev server with hot reload
dev:
	air

## Run database migrations
migrate:
	goose -dir db/migrations sqlite3 "$(DATABASE_PATH)" up

## One-time setup: run migrations and import data if DB is empty
setup:
	@mkdir -p data
	goose -dir db/migrations sqlite3 "$(DATABASE_PATH)" up
	@count=$$(sqlite3 "$(DATABASE_PATH)" "SELECT count(*) FROM pokemon;" 2>/dev/null || echo "0"); \
	if [ "$$count" = "0" ] || [ "$$count" = "" ]; then \
		echo "Database empty — importing data..."; \
		go run ./cmd/import/ --database-path "$(DATABASE_PATH)" --games frlg,hgss --seed-trainers; \
	else \
		echo "Data already imported — skipping (use 'make import' to force re-import)"; \
	fi

## Import PokéAPI data + trainer seeds (re-runnable, deletes and reloads)
import:
	go run ./cmd/import/ --database-path "$(DATABASE_PATH)" --games frlg,hgss --seed-trainers

## Build production binary
build: generate
	go build -o bin/server ./cmd/server

## Remove build artifacts
clean:
	rm -rf bin/ tmp/
