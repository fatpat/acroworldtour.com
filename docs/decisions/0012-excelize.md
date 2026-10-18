# 0012. excelize for CIVL xlsx

Status: accepted
Date: 2026-10-18

## Context

The CIVL export and import use xlsx files (spec 04, spec 06).

## Decision

Use `github.com/xuri/excelize/v2`: maintained, widely used, BSD-3-Clause licensed.

## Consequences

- No cgo; reads and writes xlsx in pure Go.
- Added with a one-line justification in the commit that first imports it.
