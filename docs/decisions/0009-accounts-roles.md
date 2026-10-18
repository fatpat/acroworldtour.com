# 0009. Accounts and roles

Status: accepted
Date: 2026-10-18

## Context

Today a single shared admin account exists (spec 08 §2.2).

## Decision

Per-person accounts with two roles: `admin` and `scorer`. Scorers are assigned per competition.

## Consequences

- Every action can be attributed to a person.
- Several scorers can work on one competition.
- A read-only role can be added later by a new record.
