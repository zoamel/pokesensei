# Replace Tailwind CSS with Modern CSS

**Date:** 2026-03-07
**Status:** Complete

## Goal

Remove Tailwind CSS and replace it with hand-written CSS using modern features for organization and maintainability.

## Decisions

- Multi-file organization with native `@import` and `@layer`
- Minimal design tokens (add as needed)
- Flat structure in `static/css/`
- Use modern CSS features: nesting, `oklch()`, container queries, `has()`/`is()`/`not()`, logical properties, `light-dark()`

## What Changes

### Remove

- Tailwind CSS standalone CLI dependency
- `static/css/input.css` (Tailwind entry point)
- `static/css/output.css` (generated, gitignored)
- Tailwind-related Makefile targets (`make tailwind`, watch mode in `make dev`)
- Tailwind class names from `.templ` files

### Add

- `static/css/main.css` -- entry point, defines `@layer` order, imports partials
- `static/css/base.css` -- reset, custom properties (design tokens), `light-dark()` theme
- `static/css/layout.css` -- page-level layout patterns
- `static/css/components.css` -- component styles (empty initially)

## File Structure

```
static/css/
  main.css          -- @layer order + @import of partials
  base.css          -- @layer base: reset, tokens, color-scheme
  layout.css        -- @layer layout: page layouts
  components.css    -- @layer components: (empty, ready for use)
```

## CSS Architecture

### main.css -- cascade control

```css
@layer base, layout, components, utilities;

@import "base.css" layer(base);
@import "layout.css" layer(layout);
@import "components.css" layer(components);
```

### base.css -- design tokens + reset

```css
:root {
  color-scheme: light dark;

  --color-text: light-dark(oklch(0.25 0.01 260), oklch(0.9 0.01 260));
  --color-text-muted: light-dark(oklch(0.45 0.01 260), oklch(0.65 0.01 260));
  --font-size-lg: 1.125rem;
  --font-size-4xl: 2.25rem;
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

### layout.css -- semantic layout classes

```css
.page-center {
  min-block-size: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}
```

### Template changes

```
// Before: <div class="min-h-screen flex items-center justify-center">
// After:  <div class="page-center">
```

## Modern CSS Features

| Feature | Where | Purpose |
|---------|-------|---------|
| `@layer` | main.css | Cascade control across files |
| `@import` with `layer()` | main.css | Multi-file + layer assignment |
| CSS nesting | All partials | Cleaner selectors |
| `oklch()` | base.css tokens | Perceptually uniform colors |
| `light-dark()` | base.css tokens | Native dark mode |
| Logical properties | layout.css | `min-block-size` etc. instead of physical properties |
| `has()`/`is()`/`not()` | As needed | Relational styling |
| Container queries | As needed | Component-level responsiveness |

## Build Pipeline Changes

- Remove `make tailwind` target and Tailwind watch from `make dev`
- Remove `tailwindcss` from `make tools`
- Update `make generate` to drop the Tailwind step
- Update layout template to reference `main.css` instead of `output.css`
- No CSS build step needed -- browsers support `@import` with `layer()` natively
