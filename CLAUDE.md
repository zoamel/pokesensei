# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Full-stack Go web application (`zoamel/pokesensei`) — PokéSensei, an app for managing Pokémon. Uses Go 1.25.5. No web framework — uses the standard library `net/http` with Go 1.22+ enhanced ServeMux for method-based routing.

**Stack:** templ (HTML templates), HTMX + Alpine.js (interactivity), Modern CSS with @layer/@import (styling), SQLite with modernc.org/sqlite (database), sqlc (query generation), goose (migrations), log/slog (structured logging).

## Build & Development Commands

```bash
make tools              # Install dev tools (templ, sqlc, goose, air)
make generate           # Run all code generators (templ + sqlc)
make dev                # Start air hot reload server
make migrate            # Run goose migrations
make setup              # One-time setup: migrations + import data
make build              # Full production build → bin/server
make clean              # Remove build artifacts

go test ./...                              # Run all tests
go test ./internal/handler/ -run TestName  # Run a single test
go build ./cmd/server/                     # Build the server binary
```

## Architecture

Layer-based with `internal/` packages. Constructor-based dependency injection. Composition root in `cmd/server/main.go`.

```
cmd/server/main.go          ← DI wiring, graceful shutdown (only file importing all packages)
internal/
  config/                    ← Config struct, LoadFromEnv() from env vars
  server/                    ← HTTP server, ServeMux routing, middleware (logging, recovery)
  handler/                   ← HTTP handlers implementing http.Handler
  database/                  ← sql.DB creation (SQLite), goose migration runner
  view/                      ← templ components (.templ files → generated _templ.go)
db/
  migrations/                ← Goose SQL migration files (SQLite dialect)
  queries/                   ← sqlc SQL query files
  generated/                 ← sqlc generated Go code (committed)
  embed.go                   ← Embeds migrations into binary via embed.FS
static/                      ← Vendored HTMX/Alpine.js, CSS (main.css + @layer partials)
data/                        ← SQLite database file (gitignored)
```

**Key patterns:**
- Handlers depend on interfaces, not concrete types (e.g., `HealthChecker` interface satisfied by sqlc `*Queries`)
- `internal/` packages never import each other horizontally — dependencies flow down from main.go
- `db/migrations/` serves as both goose migration source and sqlc schema source (single source of truth)

## Code Generation Workflow

After changing `.templ` files or `.sql` queries:

```bash
templ generate           # .templ → _templ.go (committed)
sqlc generate            # .sql → db/generated/*.go (committed)
```

Or run all at once: `make generate`

CSS files in `static/css/` are hand-written (no build step). Entry point is `main.css` which imports partials via `@import` with `@layer`.

## Configuration

Environment variables (loaded in `internal/config/`):
- `DATABASE_PATH` (default: data/pokesensei.db) — SQLite database file path
- `PORT` (default: 8080) — HTTP server port
- `LOG_LEVEL` (default: info) — debug, info, warn, error

Copy `.env.example` to `.env` for local development.
