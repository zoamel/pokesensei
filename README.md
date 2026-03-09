# PokeSensei

A full-stack web application for managing Pokemon, built with Go and zero JavaScript frameworks.

## Tech Stack

- **Go 1.25** — standard library `net/http` with Go 1.22+ enhanced ServeMux (no web framework)
- **[templ](https://templ.guide)** — type-safe HTML templates compiled to Go
- **[HTMX](https://htmx.org) + [Alpine.js](https://alpinejs.dev)** — interactivity without a JS build step
- **[SQLite](https://sqlite.org)** via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGO required
- **[sqlc](https://sqlc.dev)** — SQL queries compiled to type-safe Go code
- **[goose](https://pressly.github.io/goose/)** — database migrations
- **Modern CSS** — `@layer`, `@import`, `oklch()`, `light-dark()`, no build step

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [SQLite3 CLI](https://sqlite.org/download.html) (used by the setup script)

## Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/zoamel/pokesensei.git
cd pokesensei

# 2. Install dev tools (templ, sqlc, goose, air)
make tools

# 3. Copy environment config
cp .env.example .env

# 4. Run migrations and import Pokemon data from PokeAPI
make setup

# 5. Start the dev server with hot reload
make dev
```

The app will be available at [http://localhost:8080](http://localhost:8080).

## Available Commands

| Command          | Description                                        |
| ---------------- | -------------------------------------------------- |
| `make tools`     | Install dev tools (templ, sqlc, goose, air)        |
| `make generate`  | Run all code generators (templ + sqlc)             |
| `make dev`       | Start dev server with hot reload via air           |
| `make migrate`   | Run database migrations                            |
| `make setup`     | One-time setup: migrations + data import           |
| `make import`    | Re-import Pokemon data from PokeAPI                |
| `make build`     | Full production build to `bin/server`              |
| `make clean`     | Remove build artifacts                             |

## Configuration

Environment variables (set in `.env` or export directly):

| Variable        | Default               | Description                      |
| --------------- | --------------------- | -------------------------------- |
| `DATABASE_PATH` | `data/pokesensei.db`  | SQLite database file path        |
| `PORT`          | `8080`                | HTTP server port                 |
| `LOG_LEVEL`     | `info`                | Log level: debug, info, warn, error |

## Project Structure

```
cmd/server/main.go          # Composition root, DI wiring, graceful shutdown
internal/
  config/                    # Config loaded from environment variables
  server/                    # HTTP server, routing, middleware
  handler/                   # HTTP handlers (depend on interfaces)
  database/                  # Database connection and migration runner
  view/                      # templ components (.templ files)
db/
  migrations/                # Goose SQL migrations (also used as sqlc schema)
  queries/                   # sqlc SQL query definitions
  generated/                 # sqlc generated Go code (committed)
static/                      # Vendored JS libs, CSS partials
data/                        # SQLite database file (gitignored)
```

## Running Tests

```bash
go test ./...                              # All tests
go test ./internal/handler/ -run TestName  # Single test
```
