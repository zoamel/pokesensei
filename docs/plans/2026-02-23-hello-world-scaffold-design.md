# Hello World Project Scaffold Design

## Overview

Full-stack Go web application scaffold for "pokesensei". Sets up the foundational architecture, tooling, and a working hello world that proves all layers are wired end-to-end.

## Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25.5, standard library `net/http` |
| Templates | templ |
| Interactivity | HTMX + Alpine.js |
| Styling | Tailwind CSS (standalone CLI) |
| Database | PostgreSQL 17 with pgx/v5 driver |
| Query generation | sqlc |
| Migrations | goose |
| Logging | log/slog (stdlib) |
| Config | Environment variables |
| Dev tooling | Docker Compose (Postgres), air (hot reload) |

No web framework. No ORM.

## Architecture

Layer-based with `internal/` packages. Constructor-based dependency injection. Composition root in `main.go`.

### Directory Structure

```
pokesensei/
├── cmd/server/main.go              # Entry point, DI wiring, graceful shutdown
├── internal/
│   ├── config/config.go            # Config struct, LoadFromEnv()
│   ├── server/
│   │   ├── server.go               # Server struct, routes, ListenAndServe
│   │   └── middleware.go           # Logging, recovery middleware
│   ├── handler/
│   │   ├── home.go                 # HomeHandler (renders hello world page)
│   │   └── health.go               # HealthHandler (pings DB, returns status)
│   ├── database/
│   │   └── database.go             # NewPool() -> *pgxpool.Pool, RunMigrations()
│   └── view/
│       ├── layout.templ            # Base HTML layout (head, body, scripts)
│       ├── home.templ              # Home page content
│       └── health.templ            # Health status (optional, could be JSON-only)
├── db/
│   ├── migrations/
│   │   └── 001_initial.sql         # Goose migration placeholder
│   ├── queries/
│   │   └── health.sql              # sqlc: SELECT 1 AS ok
│   └── generated/                  # sqlc generated code
├── static/
│   ├── css/
│   │   ├── input.css               # Tailwind @import directives
│   │   └── output.css              # Compiled Tailwind output
│   └── js/                         # Vendored HTMX + Alpine.js minified
├── sqlc.yaml
├── tailwind.config.js
├── .air.toml
├── docker-compose.yml
├── .env.example
├── Makefile
└── go.mod / go.sum
```

### Dependency Flow

```
main.go (composition root)
  ├─ config.LoadFromEnv()            → Config
  ├─ database.NewPool(cfg)           → *pgxpool.Pool
  ├─ database.RunMigrations(cfg)     → runs goose
  ├─ handler.NewHome(logger)         → HomeHandler
  ├─ handler.NewHealth(pool, logger) → HealthHandler
  └─ server.New(cfg, logger, handlers...)
       └─ server.Start()             → http.ListenAndServe
```

Handlers depend on interfaces, not concrete types. Example:

```go
// handler/health.go
type DBPinger interface {
    Ping(ctx context.Context) error
}
```

`*pgxpool.Pool` satisfies `DBPinger` without an adapter.

### Routes

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | HomeHandler | Renders hello world page via templ |
| GET | `/health` | HealthHandler | Pings DB, returns health status |
| GET | `/static/*` | FileServer | Serves CSS, JS assets |

## Templates

Base layout uses templ's `children` pattern:

```templ
templ Layout(title string) {
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <title>{ title }</title>
        <link rel="stylesheet" href="/static/css/output.css"/>
        <script src="/static/js/htmx.min.js"></script>
        <script src="/static/js/alpine.min.js" defer></script>
    </head>
    <body>
        { children... }
    </body>
    </html>
}
```

Page components wrap content in `@Layout("title") { ... }`.

## Frontend Assets

- **HTMX + Alpine.js**: vendored minified JS in `static/js/` (no npm runtime dependency)
- **Tailwind CSS**: standalone CLI scans `.templ` files, outputs to `static/css/output.css`
- `tailwind.config.js` content paths: `["./internal/view/**/*.templ"]`

## Database

### sqlc config (`sqlc.yaml`)

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries/"
    schema: "db/migrations/"
    gen:
      go:
        package: "generated"
        out: "db/generated"
        sql_package: "pgx/v5"
```

`schema` points to migration files — single source of truth.

### Migrations

Goose SQL migrations in `db/migrations/`. Run at app startup via `database.RunMigrations()`.

## Dev Tooling

### Docker Compose

Postgres 17 with `pokesensei` user/password/db on port 5432.

### air (hot reload)

Watches `.go`, `.templ`, `.sql` files. Runs `templ generate` as pre-build command.

### Makefile targets

- `make dev` — start Postgres + air hot reload
- `make generate` — templ generate + sqlc generate + tailwind build
- `make migrate` — run goose migrations
- `make build` — full production build

### Environment

`.env.example` with `DATABASE_URL`, `PORT`, `LOG_LEVEL`.

## Graceful Shutdown

`main.go` listens for SIGINT/SIGTERM, calls `server.Shutdown(ctx)` and `pool.Close()`.

## Key Design Decisions

1. **Layer-based over feature-based**: simpler for a starting point, easy to refactor to vertical slices later since handlers depend on interfaces.
2. **No web framework**: Go 1.22+ enhanced ServeMux with method routing (`"GET /health"`) covers our needs.
3. **Interfaces at handler boundaries**: enables testing with mocks and future refactoring.
4. **Migrations as schema source for sqlc**: avoids duplicate schema definitions.
5. **Vendored frontend JS**: no Node.js runtime dependency. Only Tailwind CLI needed for CSS compilation.
