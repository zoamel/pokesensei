# Rename my-sundry → PokéSensei

**Date:** 2026-03-07

## Summary

Rename the application from "my-sundry" to "PokéSensei" across all source code, infrastructure, and documentation. The app will be for managing Pokémon.

## Naming Convention

- **Technical identifiers** (Go module, DB, docker): `pokesensei`
- **Display name** (page title, UI text): `PokéSensei`

## Changes

### Source Code
- `go.mod`: module `zoamel/my-sundry` → `zoamel/pokesensei`
- All Go imports referencing `zoamel/my-sundry` → `zoamel/pokesensei`
- `internal/view/home.templ`: display name `"My Sundry"` → `"PokéSensei"`
- Regenerate `internal/view/home_templ.go` via `templ generate`

### Infrastructure
- `docker-compose.yml`: user/password `sundry` → `pokesensei`, db `sundry_dev` → `pokesensei_dev`
- `.env.example`: connection string updated to match

### Documentation
- `CLAUDE.md`, `docs/technical-reference.md`, all plan docs updated
- Auto-memory `MEMORY.md` updated
