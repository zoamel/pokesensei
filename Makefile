-include .env
export

.PHONY: tools generate templ sqlc dev migrate build clean

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

## Build production binary
build: generate
	go build -o bin/server ./cmd/server

## Remove build artifacts
clean:
	rm -rf bin/ tmp/
