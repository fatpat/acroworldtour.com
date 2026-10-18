# 0003. Historical results are frozen

Status: accepted
Date: 2026-10-18

## Context

Imported competitions are official results already sent to CIVL. The legacy engine has known quirks (spec 03 §14) that the new engine fixes for new flights.

## Decision

Imported `final_marks` and legacy-computed rankings are kept as stored. Flights of imported competitions are never rescored.

## Consequences

- Fixing a scoring bug never changes past results.
- Golden tests compare the new engine with the legacy engine only on the unchanged paths.
- Imported competitions are read-only for scoring.
