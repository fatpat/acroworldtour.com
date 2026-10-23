# 0017. Per-environment bucket prefix

Status: accepted
Date: 2026-10-23

## Context

Each environment (prod, training) writes two things to Object Storage (spec 08 §6.1): the Litestream replica of its SQLite database and the uploaded files (photos, logos, PDFs). The replica path was `litestream/<AWT_ENV>/` and uploads were keyed `files/<sha256>` at the top of the bucket, so the replica path differed per environment but the uploads did not: prod and training could not share a bucket.

The owner decided on 2026-10-23: a configurable prefix per environment, `AWT_BUCKET_PREFIX` (for example `prod/` and `training/`), under which go both the files (`<prefix>files/`) and the replica (`<prefix>litestream`). It may be empty. This allows either a dedicated bucket per environment or one shared bucket.

## Decision

- `AWT_BUCKET_PREFIX` is optional. Uploads are stored at `<prefix>files/<sha256>`, the Litestream replica at `<prefix>litestream`. `AWT_ENV` is no longer part of any bucket path.
- Normalisation and validation live in one place, `internal/config` (`ParseBucketPrefix`): accepted with or without leading/trailing `/`; segments of `[A-Za-z0-9._-]` separated by single `/`; `.` and `..` segments and `//` are refused, and the app refuses to start. The result is empty or ends with exactly one `/`.
- Litestream reads `litestream.yml` with `${VAR}` expansion and cannot compute the path itself. The container entrypoint asks the binary: `awt bucket-path litestream` prints `<prefix>litestream` (or fails on an invalid prefix, which stops the container before any restore) and the entrypoint exports it as `LITESTREAM_REPLICA_PATH`. The same command is used wherever Litestream runs outside the entrypoint (weekly restore test, `init-vps.sh` replica check, restore test, point-in-time restore), so no shell copy of the normalisation drives a path. `init-vps.sh` validates its `PROD_BUCKET_PREFIX` / `TRAINING_BUCKET_PREFIX` inputs (defaults `prod/`, `training/`) with the same rule to report errors before changing anything.

## Consequences

- One bucket can serve both environments (distinct prefixes), or each has its own (any prefix, or empty).
- Existing buckets need a one-off move: root-level files to `<prefix>files/` (`docs/ops.md` section 6). The replica needs no move: on a running server the volume holds the database, so Litestream starts a fresh replica at the new path; the old `litestream/<env>/` objects can be deleted once it exists. Until then a restore on an empty volume would find no replica.
- A Litestream command run without `LITESTREAM_REPLICA_PATH` would point at the bucket root; every documented command sets it from `awt bucket-path litestream`.
