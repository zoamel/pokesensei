# Repository Guidelines

## Project Structure & Module Organization
PokeSensei is a Go web app with a thin `cmd` layer and most logic in `internal`.

- `cmd/server/main.go`: app entrypoint and dependency wiring.
- `cmd/import/`: data import pipeline (PokeAPI + trainer seed data).
- `internal/config`, `internal/server`, `internal/handler`, `internal/database`, `internal/view`: runtime configuration, HTTP stack, handlers, DB integration, and templ views.
- `db/migrations`, `db/queries`, `db/generated`: schema evolution, sqlc query definitions, and generated query code.
- `static/css`, `static/js`: frontend assets (no JS build system).
- `data/`: SQLite database files (local/dev state).

## Build, Test, and Development Commands
Use `make` targets as the default interface:

- `make tools`: install required dev tools (`templ`, `sqlc`, `goose`, `air`).
- `make setup`: run migrations and import initial game data.
- `make dev`: start hot-reload development server (`air`).
- `make generate`: regenerate templ and sqlc artifacts.
- `make migrate`: apply DB migrations to `DATABASE_PATH`.
- `make import`: re-import game data (destructive reload flow for app data).
- `make build`: generate code and build `bin/server`.
- `go test ./...`: run all tests.

## Coding Style & Naming Conventions
- Follow standard Go formatting: run `gofmt` (or editor auto-format) before commit.
- Keep package boundaries aligned with `internal/*` domain responsibilities.
- Use descriptive exported names and short, clear local identifiers.
- Tests live in `*_test.go`; generated files (`*_templ.go`, `db/generated/*.go`) should not be hand-edited.
- Keep SQL changes in `db/migrations` and matching sqlc query updates in `db/queries`.

## Testing Guidelines
- Primary framework: Go `testing` package.
- Run `go test ./...` before opening a PR.
- Scope tests by package when iterating, for example:
  - `go test ./internal/handler -run TestTeam`
- Prefer table-driven tests for handlers and config parsing where practical.

## Commit & Pull Request Guidelines
- Match existing commit style: Conventional Commit prefixes such as `feat:`, `fix:`, `docs:`, `test:`, `chore:`.
- Keep commits focused and atomic (schema, generated code, and feature code in coherent units).
- PRs should include:
  - Clear summary of behavior changes.
  - Linked issue/design doc when applicable.
  - Test evidence (`go test ./...` output summary).
  - Screenshots for UI/template/CSS changes.

## Security & Configuration Tips
- Copy `.env.example` to `.env` for local setup.
- Do not commit secrets or local DB artifacts.
- Validate migrations and imports against a fresh `data/pokesensei.db` when changing schema/data flows.
