# 0006. Legacy bugs fixed in the rewrite

Status: accepted
Date: 2026-10-18

## Context

Spec 03 §14 and spec 08 §2.1 list legacy behaviours that are clearly bugs.

## Decision

Fixed:
- `pilots_to_qualify` keeps the best N (#1).
- The previous run order is not mutated (#2).
- Per-run ranks are stored per ranking list (#3-4).
- `last_maneuver` warns when the trick is *not* in the last N (#6).
- Later runs are rescored on save (#14).
- Flights are only saved in an `open` run (#15).
- An explicit null mark means no mark, and marks must be within 0-10 (#12-13).
- Position rules read the run's catalog snapshot year (#16).
- Mark type (classic / awt / awq) is derived per pilot from the competition seasons and the pilot's AWT status instead of being chosen by the operator in three separate tabs (spec 08 §2.2).

## Consequences

- Each fix has a golden-test exception that names this record.
- New flights may differ from what the legacy engine would give.
