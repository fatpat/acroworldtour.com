# 0005. Legacy behaviours kept until the owner decides

Status: accepted
Date: 2026-10-18

## Context

Spec 08 §2.1 lists items marked "Decide" that need the sporting code or the owner.

## Decision

Keep the legacy behaviour for now:
- unpublished flights count in competition totals (#5);
- warning penalty is cumulative (#7);
- automatic position warnings are not carried to later runs (#8);
- synchro percentages are 20/20/20/40 (#18).

## Consequences

- These items are data or flags the engine can change without a rewrite.
- Each one needs a new decision record when the owner decides.
