# 0004. Repetition reset considers runs 0..run_i only

Status: accepted
Date: 2026-10-18

## Context

Spec 03 §14 #9: the legacy reset policy scans all runs, so a run created after run_i can set `first_run` and re-saving an earlier flight ignores earlier repetitions.

## Decision

The repetition reset policy only considers runs `0..run_i`. Golden cases are generated with the competition truncated to `run_i+1` runs, which is how flights were originally scored.

## Consequences

- Scoring a flight never depends on later runs.
- Golden generation must truncate the competition before calling the legacy engine.
