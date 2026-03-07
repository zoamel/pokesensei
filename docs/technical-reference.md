# Technical Reference

A learning reference for the pokesensei project. Explains the tools, patterns, Go language features, and architectural decisions behind the scaffold.

---

## 1. Tools & Their Roles

### templ — HTML Templating

**What it does:** templ is a Go-specific HTML templating language. You write `.templ` files with a syntax that looks like JSX, and the `templ generate` CLI compiles them into plain Go functions that return `templ.Component` (which implements `io.WriterTo`).

**Why templ over `html/template`:** Go's built-in `html/template` has no type safety — you pass `interface{}` data and hope the template field names match. templ generates actual Go functions with typed parameters. If you rename a field, you get a compile error, not a runtime blank. It also supports component composition (passing components as children), which `html/template` can't do.

**How it fits:** templ components live in `internal/view/`. Handlers call them directly:

```go
// internal/handler/home.go:19
view.HomePage().Render(r.Context(), w)
```

The generated `_templ.go` files are committed to git so the project builds with just `go build` (no templ CLI needed at build time).

**Key files:**
- `internal/view/layout.templ` — base HTML layout using the `{ children... }` slot pattern
- `internal/view/home.templ` — page component that composes `Layout`
- `internal/view/*_templ.go` — generated Go code (committed)

### sqlc — Type-Safe SQL

**What it does:** You write plain SQL queries in `.sql` files with a special comment (`-- name: Ping :one`), and sqlc generates Go functions with proper types. No ORM, no query builder — just SQL in, Go out.

**Why sqlc over an ORM (GORM, Ent, etc.):** ORMs hide the SQL, which means you lose control over query performance and can't use Postgres-specific features easily. sqlc keeps SQL visible and generates code that's as efficient as hand-written database calls. The generated code is also very readable — you can open `db/generated/health.sql.go` and see exactly what query runs.

**Why sqlc over hand-writing database code:** Boilerplate. Every query needs a function that prepares the statement, executes it, scans rows into structs, and handles errors. sqlc generates all of that from one SQL statement.

**How it fits:** SQL files live in `db/queries/`. Schema comes from goose migrations in `db/migrations/` (single source of truth). Generated code goes to `db/generated/`.

**Configuration (`sqlc.yaml`):**
```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries/"
    schema: "db/migrations/"    # <-- reads goose migrations as schema
    gen:
      go:
        package: "generated"
        out: "db/generated"
        sql_package: "pgx/v5"   # <-- generates pgx-native code, not database/sql
```

The `schema: "db/migrations/"` line is important — it means sqlc reads the goose migration files to understand the database schema. No separate `schema.sql` file needed.

**Key files:**
- `db/queries/health.sql` — SQL query definitions
- `db/generated/db.go` — `DBTX` interface + `Queries` struct
- `db/generated/health.sql.go` — generated `Ping()` method

### goose — Database Migrations

**What it does:** Manages database schema changes via numbered SQL files. Each file has `-- +goose Up` and `-- +goose Down` sections. `goose up` applies pending migrations, `goose down` rolls them back.

**Why goose over golang-migrate or Atlas:** goose is Go-native, supports embedding migrations into the binary via `embed.FS` (so the production binary carries its own migrations), and pairs naturally with sqlc since both read the same SQL files.

**How migrations run:** Two ways:
1. **CLI** (for development): `make migrate` runs `goose -dir db/migrations postgres "$DATABASE_URL" up`
2. **Programmatically** (at app startup): `database.RunMigrations()` uses goose's Go API with an embedded filesystem

```go
// internal/database/database.go:28-43
func RunMigrations(databaseURL string, migrationsFS fs.FS) error {
    db, err := sql.Open("pgx", databaseURL)   // uses database/sql (goose requires it)
    // ...
    goose.SetBaseFS(migrationsFS)              // use the embedded FS
    goose.Up(db, "migrations")                 // "migrations" = path within the FS
}
```

Note: goose uses `database/sql` internally, which is why `RunMigrations` opens a `sql.Open("pgx", ...)` connection — separate from the pgx pool that the app uses. The `_ "github.com/jackc/pgx/v5/stdlib"` import registers the "pgx" driver for `database/sql`.

**Key files:**
- `db/migrations/001_initial.sql` — migration files (goose format)
- `db/embed.go` — embeds migrations into the binary
- `internal/database/database.go` — `RunMigrations()` function

### pgx/v5 — PostgreSQL Driver

**What it does:** Pure Go PostgreSQL driver. We use two of its packages:

1. **`pgxpool`** — connection pool. Creates and manages a pool of database connections. This is what the app uses for all queries.
2. **`pgx/v5/stdlib`** — registers pgx as a `database/sql` driver (used only by goose for migrations).

**Why pgx over lib/pq:** lib/pq is in maintenance mode. pgx is actively developed, faster, and supports Postgres-specific features (COPY, LISTEN/NOTIFY, custom types) that lib/pq doesn't. It's also the default driver for sqlc's PostgreSQL codegen.

**How it fits:**
- `database.NewPool()` creates a `*pgxpool.Pool` — this is the app's database connection
- `generated.New(pool)` accepts the pool because `*pgxpool.Pool` satisfies sqlc's `DBTX` interface
- The pool is passed to handlers via DI, never imported directly

### HTMX — Server-Driven Interactivity

**What it does:** HTMX lets HTML elements make HTTP requests and swap content without writing JavaScript. You add attributes like `hx-get="/fragments/counter"` to an element, and HTMX handles the request, receives HTML from the server, and swaps it into the DOM.

**Why HTMX:** It keeps the rendering on the server (in Go/templ), which means no client-side state management, no API serialization layer, and no JavaScript build step. The server returns HTML fragments, not JSON.

**How it fits:** Vendored as `static/js/htmx.min.js` (v2.0.4). Loaded in the base layout. Not used in the hello world yet, but the infrastructure is wired for future features.

### Alpine.js — Lightweight Client-Side State

**What it does:** Alpine.js provides reactive behavior for HTML elements — things like toggles, dropdowns, modals, and client-side filtering. It's like a minimal Vue.js that works directly in HTML attributes (`x-data`, `x-show`, `x-on:click`).

**Why Alpine.js alongside HTMX:** HTMX handles server communication; Alpine handles purely client-side interactions (opening a dropdown, toggling a sidebar, form validation before submit). They complement each other — HTMX for server round-trips, Alpine for instant UI reactions.

**How it fits:** Vendored as `static/js/alpine.min.js` (v3.14.3). Loaded with `defer` in the base layout. Not used in the hello world yet.

### Tailwind CSS — Utility-First Styling

**What it does:** Instead of writing CSS classes like `.card-header`, you compose utility classes directly in HTML: `class="text-xl font-bold p-4"`. Tailwind's CLI scans your source files for class usage and generates only the CSS you actually use.

**Why Tailwind:** No context-switching between HTML and CSS files. No naming bikeshedding. The compiled CSS contains only what's used, so the output is small. Works especially well with templ since classes are co-located with the HTML structure.

**How it fits:**
- `tailwind.config.js` tells the CLI to scan `internal/view/**/*.templ` files
- `static/css/input.css` has Tailwind directives (`@tailwind base; @tailwind components; @tailwind utilities;`)
- The standalone CLI compiles to `static/css/output.css` (gitignored, regenerated via `make tailwind`)

### air — Hot Reload

**What it does:** Watches your project files for changes, runs a pre-build command (templ generate), rebuilds the Go binary, and restarts the server. You change a `.templ` file, save, and the browser shows the update within a second or two.

**Configuration (`.air.toml`):**
- Watches `.go` and `.templ` files
- Runs `templ generate` before each build (so template changes trigger code regeneration)
- Excludes `tmp/`, `static/`, and `db/generated/` to avoid infinite rebuild loops
- Builds to `./tmp/server` (the `tmp/` directory is gitignored)

### Docker Compose — Development Database

**What it does:** Runs PostgreSQL 17 in a container so you don't need to install Postgres locally.

```bash
docker compose up -d     # start Postgres in background
docker compose down      # stop and remove container (data persists in volume)
docker compose down -v   # stop and DELETE all data
```

The `pgdata` named volume persists data between container restarts.

---

## 2. Directory Structure

### `cmd/server/`

Go convention for application entry points. Each subdirectory of `cmd/` produces a separate binary. We have one: the HTTP server.

**Why `cmd/server/main.go` instead of just `main.go`?** Two reasons:
1. You might add more binaries later (e.g., `cmd/worker/main.go` for background jobs, `cmd/migrate/main.go` for a standalone migration tool)
2. It keeps the project root clean — config files at root, Go code in subdirectories

### `internal/`

Go's **access control mechanism**. Packages under `internal/` can only be imported by code within the same module. This prevents external packages from depending on your internal implementation details.

**Why it matters:** If someone imports `zoamel/pokesensei/internal/handler`, Go's compiler will refuse. This gives you freedom to refactor internal packages without worrying about breaking external consumers.

### `internal/config/`

Loads configuration from environment variables into a typed `Config` struct. Centralizes all config reading in one place — no scattered `os.Getenv()` calls throughout the codebase.

### `internal/server/`

Owns the HTTP server lifecycle: creating the `ServeMux`, registering middleware, starting/stopping the server. It does NOT know about handlers, templates, or the database — those are injected from `main.go`.

### `internal/handler/`

HTTP handlers. Each handler is a struct implementing `http.Handler` (via a `ServeHTTP` method). Handlers receive their dependencies through constructors (DI) and are registered with the server externally.

### `internal/database/`

Database infrastructure: creating the connection pool and running migrations. This is "plumbing" code — it doesn't contain business logic or queries. The actual queries come from sqlc-generated code in `db/generated/`.

### `internal/view/`

templ components (`.templ` files) and their generated Go counterparts (`_templ.go`). This is the presentation layer — it knows how to render HTML but not how to fetch data.

### `db/`

Lives outside `internal/` because:
1. sqlc generates into `db/generated/`, and the sqlc tool needs to find it
2. goose CLI expects to find migrations at a filesystem path
3. The `db/embed.go` file needs to be adjacent to `db/migrations/` for `//go:embed` to work (embed paths can't use `..`)

**Sub-directories:**
- `db/migrations/` — SQL migration files (goose format). Also serves as the schema source for sqlc.
- `db/queries/` — SQL query files for sqlc
- `db/generated/` — sqlc output (committed to git)
- `db/embed.go` — embeds migrations into the binary

### `static/`

Static assets served by `http.FileServer`. Vendored JS libraries (HTMX, Alpine.js) and Tailwind CSS output. No build step needed to serve these — they're plain files.

---

## 3. Architecture & Design Patterns

### Composition Root

**Pattern:** All dependency wiring happens in exactly one place — the `run()` function in `cmd/server/main.go`.

**Why:** This is the only file that imports concrete implementations from all packages. Every other package depends on interfaces or its own types. This means:
- You can see the entire application's wiring by reading one function
- Swapping implementations (e.g., a mock database for testing) requires changing only `main.go`
- No package has hidden knowledge of other packages' internals

```go
// cmd/server/main.go — the ONLY place all packages meet
cfg, _ := config.LoadFromEnv()
pool, _ := database.NewPool(ctx, cfg.DatabaseURL)
queries := generated.New(pool)
homeHandler := handler.NewHome(log)
healthHandler := handler.NewHealth(queries, log)    // queries satisfies HealthChecker
srv := server.New(cfg, log)
srv.Handle("GET /{$}", homeHandler)
```

### Constructor-Based Dependency Injection

**Pattern:** Every struct receives its dependencies through a constructor function (`NewXxx`). No global variables, no `init()` functions, no service locators.

```go
// internal/handler/health.go
func NewHealth(checker HealthChecker, log *slog.Logger) *HealthHandler {
    return &HealthHandler{checker: checker, log: log}
}
```

**Why this over a DI framework (Wire, Dig, etc.):** For a project this size, a DI framework adds complexity with no benefit. Constructor injection is explicit — you can trace every dependency by reading the code. DI frameworks use reflection or code generation to wire things, which hides the dependency graph.

**Why this over global variables:** Globals create hidden coupling and make testing hard. With constructor injection, tests can pass mock implementations directly:

```go
// internal/handler/health_test.go
h := NewHealth(&mockHealthChecker{}, logger)  // inject mock, no global state
```

### Interface-Based Contracts (Consumer-Side Interfaces)

**Pattern:** Interfaces are defined where they're used (at the consumer), not where they're implemented.

```go
// internal/handler/health.go — the CONSUMER defines what it needs
type HealthChecker interface {
    Ping(ctx context.Context) (int32, error)
}
```

The sqlc-generated `*Queries` struct happens to have a `Ping(ctx) (int32, error)` method, so it satisfies this interface implicitly — no `implements` keyword, no adapter code.

**Why consumer-side?** This is a Go idiom. It inverts the traditional OOP pattern where the provider declares "I implement interface X." Instead, the consumer says "I need something that can do Y." Benefits:
- The `handler` package doesn't import `db/generated` — it defines its own interface
- You can swap the implementation without changing the handler
- Tests create tiny mock structs instead of mocking an entire database layer
- Small, focused interfaces (often 1-2 methods) instead of large ones

### Middleware Chain

**Pattern:** HTTP middleware wraps handlers in layers, like an onion. Each middleware intercepts the request, does something, calls the next handler, and optionally does something with the response.

```go
// internal/server/middleware.go
func (s *Server) withMiddleware(next http.Handler) http.Handler {
    return s.requestLogger(s.recoverer(next))
}
```

**Execution order for `GET /health`:**
1. `requestLogger` — records start time, creates `responseWriter` wrapper
2. `recoverer` — sets up `defer/recover` for panics
3. `HealthHandler.ServeHTTP` — actual request handling
4. `recoverer` — if panic occurred, catches it and returns 500
5. `requestLogger` — logs method, path, status code, duration

**The `responseWriter` wrapper:**

Go's `http.ResponseWriter` doesn't expose the status code after `WriteHeader` is called. To log it, the middleware wraps the writer:

```go
type responseWriter struct {
    http.ResponseWriter    // embedded — delegates all methods to the original
    statusCode int         // captures the status code
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)   // still calls the real WriteHeader
}
```

### The `run()` Pattern

**Pattern:** Instead of putting logic directly in `main()`, use a `run() error` function:

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

**Why:** `main()` can't return errors, and `os.Exit()` doesn't run deferred functions. By putting the real logic in `run()`, you get:
- Proper error propagation with `return fmt.Errorf(...)`
- All `defer` statements execute (pool.Close(), cancel(), stop())
- A single error handling point in `main()`
- `run()` is also testable (you could call it from a test with a test database)

### Graceful Shutdown

**Pattern:** The server handles SIGINT/SIGTERM to close connections cleanly instead of dropping in-flight requests.

```go
// cmd/server/main.go:69-95
ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() { srv.Start() }()   // server runs in a goroutine

select {
case err := <-errCh:          // server crashed
case <-ctx.Done():             // signal received → shut down
}

srv.Shutdown(shutdownCtx)      // waits for in-flight requests (up to 10s)
```

**Why this matters:** Without graceful shutdown, pressing Ctrl+C kills the process immediately — active database transactions might be left in a broken state, and clients get connection-reset errors. With it, the server stops accepting new connections, waits for in-flight requests to complete, then exits cleanly.

---

## 4. Go Language Features

### `context.Context` — Request Scoping and Cancellation

Used everywhere a function might need to be cancelled or carry request-scoped data.

| Location | Usage |
|----------|-------|
| `cmd/server/main.go:30` | `context.Background()` — root context for the application |
| `cmd/server/main.go:69` | `signal.NotifyContext()` — context cancelled on SIGINT/SIGTERM |
| `cmd/server/main.go:87` | `context.WithTimeout()` — 10s deadline for shutdown |
| `internal/handler/home.go:19` | `r.Context()` — passes the request's context to templ rendering |
| `internal/handler/health.go:24` | `r.Context()` — passes to DB ping (cancelled if client disconnects) |
| `internal/database/database.go:14` | `NewPool(ctx, ...)` — pool creation respects context cancellation |

**Why `r.Context()` matters:** If a client closes their connection mid-request, the request context is cancelled. Passing it to database calls means the query gets cancelled too, freeing resources immediately instead of completing a query nobody's waiting for.

### Goroutines, Channels, and `select`

Used in `cmd/server/main.go` for concurrent server startup with graceful shutdown:

```go
errCh := make(chan error, 1)      // buffered channel (won't block sender)
go func() {                        // goroutine: runs server concurrently
    if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        errCh <- err               // send error to channel
    }
    close(errCh)                   // signal: goroutine is done
}()

select {                           // blocks until one case is ready
case err := <-errCh:              // server crashed with an error
case <-ctx.Done():                 // OS signal received
}
```

**Why `make(chan error, 1)` (buffered)?** If the main goroutine has already moved past the `select` (e.g., it received a signal), the server goroutine can still send its error without blocking forever. A buffered channel of size 1 ensures the send never blocks.

**Why `errors.Is(err, http.ErrServerClosed)`?** When `Shutdown()` is called, `ListenAndServe` returns `http.ErrServerClosed`. This is expected, not an error — so we filter it out.

### `embed.FS` — Compile-Time File Embedding

```go
// db/embed.go
//go:embed migrations/*.sql
var EmbedMigrations embed.FS
```

The `//go:embed` directive tells the Go compiler to include the matching files directly in the binary at compile time. At runtime, `EmbedMigrations` is a read-only filesystem containing the SQL files.

**Why embed?** The production binary is self-contained — it carries its own migration files. No need to deploy SQL files alongside the binary or worry about paths.

**Constraint:** `//go:embed` paths are relative to the source file and can't use `..`. That's why `embed.go` lives in `db/` (adjacent to `db/migrations/`), not in `internal/database/`.

### `signal.NotifyContext` — Signal Handling

```go
ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
defer stop()
```

Creates a child context that's automatically cancelled when the process receives SIGINT (Ctrl+C) or SIGTERM (kill command). This is newer (Go 1.16+) and cleaner than the older `signal.Notify` + channel pattern.

### `log/slog` — Structured Logging

Go's built-in structured logger (since Go 1.21). Outputs JSON in production:

```go
log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: cfg.LogLevel,
}))

log.Info("request",
    slog.String("method", r.Method),
    slog.Int("status", wrapped.statusCode),
    slog.Duration("duration", time.Since(start)),
)
// Output: {"time":"...","level":"INFO","msg":"request","method":"GET","status":200,"duration":"1.2ms"}
```

**Why slog over zerolog/zap:** slog is in the standard library (no dependency), has good enough performance for most apps, and its API is stable. The `slog.Level` type is used in our `Config` struct so log level is configurable via `LOG_LEVEL` env var.

### `http.ServeMux` Enhanced Routing (Go 1.22+)

```go
srv.Handle("GET /{$}", homeHandler)      // exact match for "/" only
srv.Handle("GET /health", healthHandler)  // exact match for "/health"
```

Go 1.22 added method-based routing and wildcards to the standard `ServeMux`. Before 1.22, you needed third-party routers (chi, gorilla/mux) for this.

**`{$}` explained:** Without `{$}`, the pattern `GET /` would match **every** GET request (because `/` is a prefix of all paths). The `{$}` anchor means "match only the root path, nothing else."

### `defer` — Cleanup Guarantees

`defer` schedules a function call to run when the enclosing function returns, regardless of how (normal return, error, panic). Used for cleanup:

| Location | What's deferred |
|----------|----------------|
| `cmd/server/main.go:54` | `pool.Close()` — closes all DB connections |
| `cmd/server/main.go:70` | `stop()` — stops signal listener |
| `cmd/server/main.go:88` | `cancel()` — cancels shutdown timeout context |
| `internal/database/database.go:33` | `db.Close()` — closes migration DB connection |
| `internal/server/middleware.go:31` | `func() { recover() }()` — catches panics |

**Execution order:** Defers execute in LIFO (last-in, first-out) order. In `run()`: cancel → stop → pool.Close → (from RunMigrations: db.Close).

### `recover()` — Panic Recovery

```go
defer func() {
    if err := recover(); err != nil {
        s.log.Error("panic recovered", slog.Any("error", err))
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}()
```

`recover()` only works inside a deferred function. It catches a panic, prevents the goroutine from crashing, and returns the panic value. Without this middleware, a panic in any handler would crash the entire server.

### Implicit Interface Satisfaction

Go interfaces are satisfied implicitly — no `implements` keyword. If a type has the right methods, it satisfies the interface:

```go
// handler defines what it needs
type HealthChecker interface {
    Ping(ctx context.Context) (int32, error)
}

// sqlc generates this method on *Queries — it satisfies HealthChecker automatically
func (q *Queries) Ping(ctx context.Context) (int32, error) { ... }
```

This means `handler` doesn't need to import `generated`. The wiring happens in `main.go`:

```go
queries := generated.New(pool)           // *generated.Queries
healthHandler := handler.NewHealth(queries, log)  // accepted as HealthChecker
```

### Struct Embedding

```go
type responseWriter struct {
    http.ResponseWriter    // embedded field — no field name
    statusCode int
}
```

Embedding promotes all methods of `http.ResponseWriter` to `responseWriter`. This means `responseWriter` implements `http.ResponseWriter` automatically — it has `Write()`, `Header()`, and `WriteHeader()` from the embedded field. We override `WriteHeader` to capture the status code while delegating to the original.

### `t.Setenv()` — Test Environment Isolation

```go
func TestLoadFromEnv_AllSet(t *testing.T) {
    t.Setenv("DATABASE_URL", "postgres://test:test@localhost/testdb")
    // ...
}
```

`t.Setenv()` (Go 1.17+) sets an environment variable for the duration of the test and automatically restores the original value when the test finishes. This prevents tests from polluting each other's environment.

### `httptest` — HTTP Testing Without a Server

```go
req := httptest.NewRequest(http.MethodGet, "/", nil)   // fake request
rec := httptest.NewRecorder()                            // fake response writer
handler.ServeHTTP(rec, req)                              // call handler directly
// inspect rec.Code, rec.Body, rec.Header()
```

For integration tests, `httptest.NewServer(handler)` starts a real HTTP server on a random port:

```go
ts := httptest.NewServer(srv.Handler())
defer ts.Close()
resp, _ := http.Get(ts.URL + "/test")   // real HTTP request
```

---

## 5. Data Flow

### `GET /` — Home Page

```
1. Client sends GET /
2. net/http dispatches to ServeMux (pattern "GET /{$}" matches)
3. requestLogger middleware: records start time, wraps ResponseWriter
4. recoverer middleware: sets up defer/recover
5. HomeHandler.ServeHTTP:
   a. Calls view.HomePage() — returns templ.Component
   b. Calls .Render(r.Context(), w) — writes HTML to ResponseWriter
   c. templ engine: HomePage calls Layout("PokéSensei") with children
   d. Layout writes: DOCTYPE, <html>, <head> (CSS, JS links), <body>
   e. HomePage children: <main> with "Hello, World!"
   f. Layout closes: </body>, </html>
6. recoverer: no panic, passes through
7. requestLogger: logs {"method":"GET","path":"/","status":200,"duration":"0.5ms"}
8. Client receives full HTML page
```

### `GET /health` — Health Check

```
1. Client sends GET /health
2. net/http dispatches to ServeMux (pattern "GET /health" matches)
3. requestLogger middleware: records start time, wraps ResponseWriter
4. recoverer middleware: sets up defer/recover
5. HealthHandler.ServeHTTP:
   a. Calls h.checker.Ping(r.Context())
      → This calls *generated.Queries.Ping()
      → Which executes: SELECT 1 AS ok (via pgxpool connection)
      → Returns (1, nil) on success
   b. Sets Content-Type: application/json
   c. json.NewEncoder(w).Encode({"status":"ok"})
6. recoverer: no panic, passes through
7. requestLogger: logs {"method":"GET","path":"/health","status":200,"duration":"1.2ms"}
8. Client receives: {"status":"ok"}
```

### Application Startup

```
1. main() calls run()
2. config.LoadFromEnv() → reads DATABASE_URL, PORT, LOG_LEVEL
3. slog.New(JSONHandler) → creates JSON logger
4. database.RunMigrations(dbURL, embedFS):
   a. sql.Open("pgx", dbURL) → database/sql connection (for goose)
   b. goose.SetBaseFS(embedFS) → use embedded migration files
   c. goose.Up(db, "migrations") → apply pending migrations
   d. db.Close() → close the migration connection
5. database.NewPool(ctx, dbURL):
   a. pgxpool.New() → create connection pool
   b. pool.Ping() → verify connectivity
6. generated.New(pool) → create sqlc Queries with the pool
7. handler.NewHome(log), handler.NewHealth(queries, log) → create handlers
8. server.New(cfg, log) → create server with middleware
9. srv.Handle(...) → register routes
10. signal.NotifyContext → listen for SIGINT/SIGTERM
11. go srv.Start() → start listening in goroutine
12. select {} → block until error or signal
13. On signal: srv.Shutdown(10s timeout) → drain connections
14. pool.Close() → close all DB connections (via defer)
15. run() returns nil → main() exits 0
```

---

## 6. Code Generation Pipeline

### What's Generated vs Hand-Written

| File | Source | Generator | Committed? |
|------|--------|-----------|-----------|
| `internal/view/*_templ.go` | `*.templ` files | `templ generate` | Yes |
| `db/generated/*.go` | `db/queries/*.sql` + `db/migrations/*.sql` | `sqlc generate` | Yes |
| `static/css/output.css` | `static/css/input.css` + `internal/view/**/*.templ` | `tailwindcss` | No (gitignored) |

**Why commit generated Go code?** So the project builds with just `go build` — no templ or sqlc CLI needed. CI doesn't need extra tools. Contributors can clone and build immediately.

**Why NOT commit generated CSS?** Tailwind output changes frequently as you add/remove CSS classes. It's fast to regenerate and would add noise to git diffs. The input file (`input.css`) and config (`tailwind.config.js`) are committed.

### Build Order

The generators have dependencies:

```
1. templ generate        (no dependencies — reads .templ, writes .go)
2. sqlc generate         (no dependencies — reads .sql, writes .go)
3. tailwindcss           (depends on .templ files existing for class scanning)
4. go build              (depends on all generated .go files)
```

Steps 1-3 can run in parallel (they don't depend on each other). `make generate` runs all three. The air hot-reload tool runs `templ generate` as a pre-build step before `go build`.

### Regenerating After Changes

| You changed... | Run... |
|----------------|--------|
| A `.templ` file | `templ generate` (or air does it automatically) |
| A SQL query in `db/queries/` | `sqlc generate` |
| A migration in `db/migrations/` | `sqlc generate` (schema changed) + `make migrate` |
| CSS classes in `.templ` files | `make tailwind` (regenerates output.css) |
| Everything | `make generate` |
