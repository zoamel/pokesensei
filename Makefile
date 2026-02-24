-include .env
export

.PHONY: tools generate templ sqlc tailwind dev migrate build clean

## Install development tools
tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/air-verse/air@latest
	@echo "NOTE: Install Tailwind CSS standalone CLI separately:"
	@echo "  https://tailwindcss.com/blog/standalone-cli"

## Run all code generators
generate: templ sqlc tailwind

## Generate templ Go code from .templ files
templ:
	templ generate

## Generate sqlc Go code from SQL queries
sqlc:
	sqlc generate

## Build Tailwind CSS
tailwind:
	tailwindcss -i static/css/input.css -o static/css/output.css --minify

## Start dev server with hot reload (requires Docker Compose running)
dev:
	@tailwindcss -i static/css/input.css -o static/css/output.css --watch & \
	air; \
	kill %1 2>/dev/null || true

## Run database migrations
migrate:
	goose -dir db/migrations postgres "$(DATABASE_URL)" up

## Build production binary
build: generate
	go build -o bin/server ./cmd/server

## Remove build artifacts
clean:
	rm -rf bin/ tmp/
