# 0013. Runtime image: alpine with a shell entrypoint

Status: accepted
Date: 2026-10-18

## Context

The container must restore the database from the Litestream replica when the volume is empty, then run the app under `litestream replicate -exec`. The brief preferred `distroless/static`, which has no shell.

## Decision

Use `alpine:3.22` (pinned by digest) as the runtime base with a small POSIX `sh` entrypoint (`deploy/entrypoint.sh`). The Litestream 0.3.13 binary is copied from its official image (it is static, so it runs on musl). The app is built with `CGO_ENABLED=0`.

Distroless would need the restore-then-replicate sequence rebuilt inside `awt` itself, duplicating Litestream's supervision. Alpine adds about 8 MB, gives a `wget` health check, and lets operators `docker exec` a shell during an incident.

## Consequences

- The entrypoint skips Litestream when `AWT_S3_BUCKET` is empty, so `docker run` works with no replica.
- The container runs as the non-root user `awt` (uid 10001).
- Base images are pinned by tag and digest; bumping them is a reviewed change.
