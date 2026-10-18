# 0002. Go, templ and htmx

Status: accepted
Date: 2026-10-18

## Context

The site must be server-rendered, work without JavaScript, and be maintainable by AI agents with strong static checking. Spec 08 §7 compares stacks.

## Decision

Use Go with `net/http`, `templ` components for HTML, and htmx (with its SSE extension) for partial updates and live results. No SPA, no npm, no bundler.

## Consequences

- One typed language end to end; templates are compile-checked.
- htmx is vendored at a pinned version.
- Small plain-JS modules cover what htmx cannot.
