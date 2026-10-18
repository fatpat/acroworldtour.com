# 0010. goldmark for Markdown

Status: accepted
Date: 2026-10-18

## Context

Institutional content is written as Markdown (spec 07 §10) and rendered server-side.

## Decision

Use `github.com/yuin/goldmark`: maintained, widely used, MIT licensed, CommonMark compliant.

## Consequences

- Raw HTML in content stays disabled by default.
- Added with a one-line justification in the commit that first imports it.
