# Replace Tailwind CSS with Modern CSS — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove Tailwind CSS and replace it with hand-written CSS using `@layer`, `@import`, `oklch()`, `light-dark()`, logical properties, and multi-file organization.

**Architecture:** Four CSS files in `static/css/` — `main.css` as the entry point that imports `base.css`, `layout.css`, and `components.css` via `@import` with `layer()`. No build step needed; browsers handle `@import` natively. Templates switch from Tailwind utility classes to semantic CSS classes.

**Tech Stack:** Modern CSS (`@layer`, `@import`, `oklch()`, `light-dark()`, logical properties, CSS nesting), templ templates, Go `net/http` static file serving.

**Design doc:** `docs/plans/2026-03-07-replace-tailwind-with-modern-css-design.md`

---

### Task 1: Create the CSS files

**Files:**
- Create: `static/css/base.css`
- Create: `static/css/layout.css`
- Create: `static/css/components.css`
- Create: `static/css/main.css`

**Step 1: Create `static/css/base.css`**

```css
:root {
  color-scheme: light dark;

  /* Colors — oklch for perceptual uniformity, light-dark() for theme switching */
  --color-text: light-dark(oklch(0.25 0.01 260), oklch(0.9 0.01 260));
  --color-text-muted: light-dark(oklch(0.45 0.01 260), oklch(0.65 0.01 260));

  /* Typography */
  --font-size-lg: 1.125rem;
  --font-size-4xl: 2.25rem;

  /* Spacing */
  --space-4: 1rem;
}

*,
*::before,
*::after {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: system-ui, sans-serif;
  color: var(--color-text);
}
```

**Step 2: Create `static/css/layout.css`**

```css
.page-center {
  min-block-size: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}
```

**Step 3: Create `static/css/components.css`**

```css
/* Component styles — add as the application grows */
```

**Step 4: Create `static/css/main.css`**

```css
@layer base, layout, components, utilities;

@import "base.css" layer(base);
@import "layout.css" layer(layout);
@import "components.css" layer(components);
```

**Step 5: Commit**

```bash
git add static/css/main.css static/css/base.css static/css/layout.css static/css/components.css
git commit -m "feat: add modern CSS files with @layer and @import"
```

---

### Task 2: Update templ templates

**Files:**
- Modify: `internal/view/layout.templ:10` — change stylesheet href
- Modify: `internal/view/home.templ:5-12` — replace Tailwind classes with semantic CSS

**Step 1: Update `internal/view/layout.templ`**

Change line 10 from:
```html
<link rel="stylesheet" href="/static/css/output.css"/>
```
to:
```html
<link rel="stylesheet" href="/static/css/main.css"/>
```

**Step 2: Update `internal/view/home.templ`**

Replace the entire template body. The new version uses semantic classes and updates the tagline text (no longer references Tailwind):

```
package view

templ HomePage() {
	@Layout("My Sundry") {
		<main class="page-center">
			<div>
				<h1 class="heading">Hello, World!</h1>
				<p class="subheading">
					Full-stack Go with templ, HTMX, and modern CSS.
				</p>
			</div>
		</main>
	}
}
```

**Step 3: Add the heading/subheading styles to `static/css/components.css`**

```css
.heading {
  font-size: var(--font-size-4xl);
  font-weight: 700;
  color: var(--color-text);
}

.subheading {
  margin-block-start: var(--space-4);
  font-size: var(--font-size-lg);
  color: var(--color-text-muted);
}
```

**Step 4: Regenerate templ**

Run: `templ generate`
Expected: generates `internal/view/layout_templ.go` and `internal/view/home_templ.go` with no errors.

**Step 5: Commit**

```bash
git add internal/view/layout.templ internal/view/layout_templ.go \
       internal/view/home.templ internal/view/home_templ.go \
       static/css/components.css
git commit -m "feat: replace Tailwind classes with semantic CSS in templates"
```

---

### Task 3: Remove Tailwind from build pipeline

**Files:**
- Delete: `static/css/input.css`
- Modify: `Makefile:4,7,12,16,26-28,31-34`
- Modify: `.gitignore:37-38`

**Step 1: Delete `static/css/input.css`**

```bash
rm static/css/input.css
```

**Step 2: Update `Makefile`**

Replace the entire Makefile with:

```makefile
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
```

Key changes:
- Removed `tailwind` from `.PHONY` and `generate` dependency
- Removed `tailwind` target entirely
- Removed Tailwind CLI install note from `tools`
- Simplified `dev` to just `air` (no Tailwind watch process)

**Step 3: Update `.gitignore`**

Remove these two lines:
```
# Tailwind output (regenerated from input.css)
static/css/output.css
```

Also delete the generated `static/css/output.css` file if it exists:
```bash
rm -f static/css/output.css
```

**Step 4: Commit**

```bash
git add -A
git commit -m "chore: remove Tailwind CSS from build pipeline"
```

---

### Task 4: Update project documentation

**Files:**
- Modify: `CLAUDE.md` — update CSS references
- Modify: `docs/plans/2026-03-07-replace-tailwind-with-modern-css-design.md` — mark complete

**Step 1: Update `CLAUDE.md`**

In the Project Overview section, replace "Tailwind CSS v4 (styling)" with "Modern CSS with @layer/@import (styling)".

In Build & Development Commands, remove:
- Any Tailwind-related commands

In Code Generation Workflow, remove the tailwindcss line and update the "Or run all at once" note.

In Architecture file tree under `static/`, update the comment to reflect the new CSS structure.

**Step 2: Mark design doc as complete**

Change `**Status:** Approved` to `**Status:** Complete` in `docs/plans/2026-03-07-replace-tailwind-with-modern-css-design.md`.

**Step 3: Commit**

```bash
git add CLAUDE.md docs/plans/2026-03-07-replace-tailwind-with-modern-css-design.md
git commit -m "docs: update project docs to reflect CSS migration"
```

---

### Task 5: Verify everything works

**Step 1: Build the project**

Run: `make generate`
Expected: templ and sqlc generate successfully, no errors.

**Step 2: Build the binary**

Run: `go build ./cmd/server/`
Expected: builds without errors.

**Step 3: Manual smoke test**

Run: `make dev` (with Postgres running via `docker compose up -d`)
Open `http://localhost:8080` in a browser.
Expected:
- Page loads with centered "Hello, World!" heading
- Subtitle reads "Full-stack Go with templ, HTMX, and modern CSS."
- Text colors work in both light and dark mode (toggle OS theme)
- No console errors about missing CSS files

**Step 4: Run tests**

Run: `go test ./...`
Expected: all tests pass.

**Step 5: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: address any issues from verification"
```
