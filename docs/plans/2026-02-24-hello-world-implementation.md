# Hello World Scaffold Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Set up a full-stack Go web application scaffold with templ, HTMX, Alpine.js, Tailwind CSS, sqlc, and PostgreSQL — proving all layers work end-to-end with a hello world page and a DB-connected health check.

**Architecture:** Layer-based with `internal/` packages. Constructor-based DI with composition root in `cmd/server/main.go`. Handlers depend on interfaces, not concrete types. See `docs/plans/2026-02-23-hello-world-scaffold-design.md` for full design.

**Tech Stack:** Go 1.25.5, net/http, templ, HTMX 2.0, Alpine.js 3.14, Tailwind CSS v3, PostgreSQL 17, pgx/v5, sqlc, goose v3, log/slog

---

### Task 1: Dev Tooling & Configuration Files

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `sqlc.yaml`
- Create: `tailwind.config.js`
- Create: `.air.toml`
- Modify: `.gitignore`

**Step 1: Create `docker-compose.yml`**

```yaml
services:
  db:
    image: postgres:17
    environment:
      POSTGRES_USER: sundry
      POSTGRES_PASSWORD: sundry
      POSTGRES_DB: sundry_dev
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

**Step 2: Create `.env.example`**

```
DATABASE_URL=postgres://sundry:sundry@localhost:5432/sundry_dev?sslmode=disable
PORT=8080
LOG_LEVEL=debug
```

**Step 3: Create `sqlc.yaml`**

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

**Step 4: Create `tailwind.config.js`**

```js
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./internal/view/**/*.templ"],
  theme: {
    extend: {},
  },
  plugins: [],
};
```

**Step 5: Create `.air.toml`**

```toml
root = "."
tmp_dir = "tmp"

[build]
  pre_cmd = ["templ generate"]
  cmd = "go build -o ./tmp/server ./cmd/server"
  bin = "./tmp/server"
  include_ext = ["go", "templ"]
  exclude_dir = ["tmp", "static", "db/generated"]
  delay = 1000
```

**Step 6: Update `.gitignore` — append these lines**

```
# Air
tmp/

# Tailwind output (regenerated from input.css)
static/css/output.css

# OS files
.DS_Store
```

**Step 7: Commit**

```bash
git add docker-compose.yml .env.example sqlc.yaml tailwind.config.js .air.toml .gitignore
git commit -m "Add dev tooling configuration files

Docker Compose for Postgres, sqlc/Tailwind/air configs,
and env var example."
```

---

### Task 2: Makefile

**Files:**
- Create: `Makefile`

**Step 1: Create `Makefile`**

```makefile
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
```

**Step 2: Commit**

```bash
git add Makefile
git commit -m "Add Makefile with dev, generate, migrate, and build targets"
```

---

### Task 3: Go Dependencies

**Files:**
- Modify: `go.mod`
- Create: `go.sum`

**Step 1: Install all Go dependencies**

```bash
go get github.com/a-h/templ@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/pressly/goose/v3@latest
```

**Step 2: Verify `go.mod` has all three dependencies**

```bash
grep -E "templ|pgx|goose" go.mod
```

Expected: three lines showing the dependencies.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "Add Go dependencies: templ, pgx/v5, goose/v3"
```

---

### Task 4: Config Package (TDD)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"log/slog"
	"testing"
)

func TestLoadFromEnv_AllSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/testdb")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://test:test@localhost/testdb" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://test:test@localhost/testdb")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/testdb")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want default %v", cfg.LogLevel, slog.LevelInfo)
	}
}

func TestLoadFromEnv_MissingDatabaseURL(t *testing.T) {
	// DATABASE_URL not set
	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}
```

**Step 2: Create minimal `config.go` to make it compile (but tests fail)**

Create `internal/config/config.go`:

```go
package config
```

**Step 3: Run tests to verify they fail**

```bash
go test ./internal/config/ -v
```

Expected: compilation errors (LoadFromEnv, Config not defined).

**Step 4: Implement `config.go`**

```go
package config

import (
	"fmt"
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	LogLevel    slog.Level
}

func LoadFromEnv() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
		LogLevel:    parseLogLevel(os.Getenv("LOG_LEVEL")),
	}, nil
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/config/ -v
```

Expected: all 3 tests PASS.

**Step 6: Commit**

```bash
git add internal/config/
git commit -m "Add config package with env var loading and tests"
```

---

### Task 5: Database Migration, sqlc Queries, and Embed

**Files:**
- Create: `db/migrations/001_initial.sql`
- Create: `db/queries/health.sql`
- Create: `db/embed.go`

**Step 1: Create migration file `db/migrations/001_initial.sql`**

```sql
-- +goose Up
SELECT 1;

-- +goose Down
-- nothing to undo
```

**Step 2: Create sqlc query file `db/queries/health.sql`**

```sql
-- name: Ping :one
SELECT 1 AS ok;
```

**Step 3: Create `db/embed.go`**

```go
package db

import "embed"

//go:embed migrations/*.sql
var EmbedMigrations embed.FS
```

**Step 4: Run sqlc generate**

```bash
sqlc generate
```

Expected: creates `db/generated/db.go`, `db/generated/models.go`, `db/generated/health.sql.go`.

**Step 5: Verify generated code compiles**

```bash
go build ./db/...
```

Expected: no errors.

**Step 6: Commit**

```bash
git add db/ sqlc.yaml
git commit -m "Add database migrations, sqlc queries, and generated code"
```

---

### Task 6: Database Package

**Files:**
- Create: `internal/database/database.go`

**Step 1: Create `internal/database/database.go`**

```go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

func RunMigrations(databaseURL string, migrationsFS fs.FS) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("opening database for migrations: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/database/
```

Expected: no errors.

**Step 3: Commit**

```bash
git add internal/database/
git commit -m "Add database package with connection pool and migration runner"
```

---

### Task 7: Static Assets

**Files:**
- Create: `static/css/input.css`
- Create: `static/js/.gitkeep` (HTMX and Alpine.js vendored files)

**Step 1: Create `static/css/input.css`**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

**Step 2: Download HTMX and Alpine.js**

```bash
mkdir -p static/js
curl -sL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js -o static/js/htmx.min.js
curl -sL https://unpkg.com/alpinejs@3.14.3/dist/cdn.min.js -o static/js/alpine.min.js
```

**Step 3: Verify files were downloaded**

```bash
head -c 100 static/js/htmx.min.js
head -c 100 static/js/alpine.min.js
```

Expected: JavaScript content (not HTML error pages).

**Step 4: Generate Tailwind CSS output**

```bash
tailwindcss -i static/css/input.css -o static/css/output.css --minify
```

If the standalone CLI is not installed yet, install it first via the instructions at https://tailwindcss.com/blog/standalone-cli or use `npx tailwindcss` as a fallback.

**Step 5: Commit**

```bash
git add static/
git commit -m "Add static assets: HTMX, Alpine.js, and Tailwind CSS"
```

---

### Task 8: Templ Templates

**Files:**
- Create: `internal/view/layout.templ`
- Create: `internal/view/home.templ`

**Step 1: Create `internal/view/layout.templ`**

```templ
package view

templ Layout(title string) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="UTF-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
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

**Step 2: Create `internal/view/home.templ`**

```templ
package view

templ HomePage() {
	@Layout("My Sundry") {
		<main class="min-h-screen flex items-center justify-center">
			<div class="text-center">
				<h1 class="text-4xl font-bold text-gray-900">Hello, World!</h1>
				<p class="mt-4 text-lg text-gray-600">
					Full-stack Go with templ, HTMX, and Tailwind CSS.
				</p>
			</div>
		</main>
	}
}
```

**Step 3: Run templ generate**

```bash
templ generate
```

Expected: creates `internal/view/layout_templ.go` and `internal/view/home_templ.go`.

**Step 4: Verify generated code compiles**

```bash
go build ./internal/view/
```

Expected: no errors.

**Step 5: Commit**

```bash
git add internal/view/
git commit -m "Add templ templates: base layout and home page"
```

---

### Task 9: Home Handler (TDD)

**Files:**
- Create: `internal/handler/home.go`
- Create: `internal/handler/home_test.go`

**Step 1: Write the failing test**

Create `internal/handler/home_test.go`:

```go
package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeHandler_GET(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHome(logger)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Hello, World!") {
		t.Errorf("body does not contain 'Hello, World!', got: %s", body[:min(len(body), 200)])
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestHomeHandler -v
```

Expected: compilation error (NewHome not defined).

**Step 3: Implement `internal/handler/home.go`**

```go
package handler

import (
	"log/slog"
	"net/http"

	"zoamel/my-sundry/internal/view"
)

type HomeHandler struct {
	log *slog.Logger
}

func NewHome(log *slog.Logger) *HomeHandler {
	return &HomeHandler{log: log}
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := view.HomePage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render home page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestHomeHandler -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/handler/home.go internal/handler/home_test.go
git commit -m "Add home handler with templ rendering and test"
```

---

### Task 10: Health Handler (TDD)

**Files:**
- Create: `internal/handler/health.go`
- Create: `internal/handler/health_test.go`

**Step 1: Write the failing test**

Create `internal/handler/health_test.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockHealthChecker struct {
	err error
}

func (m *mockHealthChecker) Ping(ctx context.Context) (int32, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 1, nil
}

func TestHealthHandler_Healthy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHealth(&mockHealthChecker{}, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestHealthHandler_Unhealthy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHealth(&mockHealthChecker{err: errors.New("connection refused")}, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("status = %q, want %q", resp["status"], "error")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestHealthHandler -v
```

Expected: compilation error (NewHealth, HealthChecker not defined).

**Step 3: Implement `internal/handler/health.go`**

```go
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

type HealthChecker interface {
	Ping(ctx context.Context) (int32, error)
}

type HealthHandler struct {
	checker HealthChecker
	log     *slog.Logger
}

func NewHealth(checker HealthChecker, log *slog.Logger) *HealthHandler {
	return &HealthHandler{checker: checker, log: log}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := h.checker.Ping(r.Context())

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		h.log.Error("health check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestHealthHandler -v
```

Expected: both tests PASS.

**Step 5: Commit**

```bash
git add internal/handler/health.go internal/handler/health_test.go
git commit -m "Add health handler with DB ping check and tests"
```

---

### Task 11: Server Package

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/middleware.go`
- Create: `internal/server/server_test.go`

**Step 1: Write the failing test**

Create `internal/server/server_test.go`:

```go
package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"zoamel/my-sundry/internal/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{Port: "0", LogLevel: slog.LevelInfo}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, logger)
}

func TestServer_RoutesRegistered(t *testing.T) {
	srv := newTestServer(t)

	// Register a test handler
	srv.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestServer_MiddlewareRecovery(t *testing.T) {
	srv := newTestServer(t)

	srv.Handle("GET /panic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/panic")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/server/ -v
```

Expected: compilation error (New, Server, Handler not defined).

**Step 3: Implement `internal/server/middleware.go`**

```go
package server

import (
	"log/slog"
	"net/http"
	"time"
)

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return s.requestLogger(s.recoverer(next))
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		s.log.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrapped.statusCode),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.log.Error("panic recovered",
					slog.Any("error", err),
					slog.String("path", r.URL.Path),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
```

**Step 4: Implement `internal/server/server.go`**

```go
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"zoamel/my-sundry/internal/config"
)

type Server struct {
	cfg        *config.Config
	log        *slog.Logger
	mux        *http.ServeMux
	httpServer *http.Server
}

func New(cfg *config.Config, log *slog.Logger) *Server {
	s := &Server{
		cfg: cfg,
		log: log,
		mux: http.NewServeMux(),
	}

	s.httpServer = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      s.withMiddleware(s.mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// Handler returns the fully wrapped HTTP handler (for testing).
func (s *Server) Handler() http.Handler {
	return s.withMiddleware(s.mux)
}

func (s *Server) Start() error {
	s.log.Info("server starting", slog.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/server/ -v
```

Expected: all tests PASS.

**Step 6: Commit**

```bash
git add internal/server/
git commit -m "Add server package with routing, middleware, and tests"
```

---

### Task 12: Application Entry Point (`main.go`)

**Files:**
- Create: `cmd/server/main.go`

**Step 1: Create `cmd/server/main.go`**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zoamel/my-sundry/db"
	"zoamel/my-sundry/db/generated"
	"zoamel/my-sundry/internal/config"
	"zoamel/my-sundry/internal/database"
	"zoamel/my-sundry/internal/handler"
	"zoamel/my-sundry/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Setup structured logger
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	// Run database migrations
	if err := database.RunMigrations(cfg.DatabaseURL, db.EmbedMigrations); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Info("migrations completed")

	// Create database connection pool
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("creating database pool: %w", err)
	}
	defer pool.Close()
	log.Info("database connected")

	// Wire dependencies
	queries := generated.New(pool)
	homeHandler := handler.NewHome(log)
	healthHandler := handler.NewHealth(queries, log)

	// Configure server and routes
	srv := server.New(cfg, log)
	srv.Handle("GET /{$}", homeHandler)
	srv.Handle("GET /health", healthHandler)
	srv.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down gracefully")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("server stopped")
	return nil
}
```

**Step 2: Verify it compiles**

```bash
go build ./cmd/server/
```

Expected: no errors. Binary created at `./server` (or current directory).

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "Add application entry point with DI wiring and graceful shutdown"
```

---

### Task 13: End-to-End Verification

**Step 1: Start Postgres**

```bash
docker compose up -d
```

Expected: Postgres container starts on port 5432.

**Step 2: Copy env file and source it**

```bash
cp .env.example .env
```

**Step 3: Run the application**

```bash
source .env && go run ./cmd/server/
```

Expected: log lines showing "migrations completed", "database connected", "server starting".

**Step 4: Test home page (in another terminal)**

```bash
curl -s http://localhost:8080/ | head -20
```

Expected: HTML containing "Hello, World!" and templ-rendered layout.

**Step 5: Test health endpoint**

```bash
curl -s http://localhost:8080/health
```

Expected: `{"status":"ok"}`

**Step 6: Test static file serving**

```bash
curl -sI http://localhost:8080/static/js/htmx.min.js
```

Expected: `200 OK` with `Content-Type` header.

**Step 7: Stop the application with Ctrl+C**

Expected: "shutting down gracefully", "server stopped" log lines.

**Step 8: Stop Postgres**

```bash
docker compose down
```

---

### Task 14: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update CLAUDE.md with project-specific guidance**

Replace contents of `CLAUDE.md` with accurate information reflecting the actual project structure, commands, and architecture. Include:

- Project overview (full-stack Go with templ/HTMX/Alpine/Tailwind/sqlc/Postgres)
- Build and dev commands (make targets, go test, etc.)
- Architecture overview (layer-based, DI, composition root)
- Code generation workflow (templ generate, sqlc generate, tailwind build)
- Key conventions (interfaces at handler boundaries, no framework, standard library)

**Step 2: Run all tests one final time**

```bash
go test ./... -v
```

Expected: all tests pass across all packages.

**Step 3: Commit everything**

```bash
git add CLAUDE.md go.mod go.sum .env.example
git commit -m "Update CLAUDE.md with full project guidance"
```
