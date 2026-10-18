# 0008. Trick catalog versioned by year

Status: accepted
Date: 2026-10-18

## Context

Coefficients and position rules change between seasons (spec 03 §14 #16, #23).

## Decision

The trick catalog is versioned by year. A competition uses the catalog of `start_date.year`.

## Consequences

- A run keeps a catalog snapshot, so old flights stay reproducible.
- A new season requires importing a new catalog version.
