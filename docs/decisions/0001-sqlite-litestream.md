# 0001. SQLite with Litestream

Status: accepted
Date: 2026-10-18

## Context

The data is relational (competitions, runs, flights, marks) and needs transactions. Write volume is low and a single scorer team writes at a time. Spec 08 §6.2 weighs managed PostgreSQL against SQLite.

## Decision

Use SQLite in WAL mode through `modernc.org/sqlite` (pure Go, no cgo). Litestream replicates the file to object storage for backup and restore.

## Consequences

- Minimal operations: one binary, one data file.
- Single writer process; the app is not scaled horizontally.
- Restore is a Litestream restore, to be rehearsed before go-live.
