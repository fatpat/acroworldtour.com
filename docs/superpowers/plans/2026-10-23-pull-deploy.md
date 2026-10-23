# Pull-based Deploy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the SSH push deploy with an agent on the VPS that pulls new images from the public GHCR package and deploys training then prod.

**Architecture:** CI only builds and pushes `ghcr.io/<owner>/awt:<sha>` and the moving tag `:rewrite-go`, with `compose.yaml` and `Caddyfile` baked into the image and the commit in an OCI label. A systemd timer on the VPS runs `deploy/agent.sh` every 60 s: it pulls the moving tag, reads the revision, installs the baked config, and calls the existing `deploy.sh` for training then prod (health check + rollback unchanged). Results go to journald only.

**Tech Stack:** POSIX sh, Docker / Docker Compose, systemd, GitHub Actions.

**Spec:** Agreed in conversation on 2026-10-23 (owner): pull-based agent; both environments updated automatically with the same image (training first, prod only if training is healthy); results in server logs only; repository and image package are public (no registry credentials on the VPS).

## Global Constraints

- No SSH from CI, no `VPS_*` GitHub secrets, no deploy key: remove the CI `deploy` job, the forced-command mode and `sync` from `deploy.sh`, and their docs.
- The image is public: the agent pulls anonymously; no `docker login`.
- Moving tag name = deploy branch name `rewrite-go`, configurable on the VPS via `AWT_CHANNEL` in `/opt/awt/.env` (default `rewrite-go`).
- Poll interval 60 s (systemd timer `OnUnitActiveSec=60s`, `OnBootSec=2min`, `Persistent=false`); one run at a time (`flock` on `/run/lock/awt-agent.lock`; a run that cannot take the lock exits 0 silently).
- Training is deployed first; prod only after training succeeded; both get the same revision.
- A revision that failed (training or prod) is recorded in `/opt/awt/state/failed-revisions` and never retried automatically; logged once per run as `skip <rev>: failed before`. A new revision deploys normally. Manual retry = delete the line.
- Logs: plain lines on stdout/stderr, collected by journald (`journalctl -u awt-agent`). No secrets in logs. No change when nothing new: one line at debug level is not needed — print nothing.
- Keep all existing pins (actions by SHA, images by digest). New images in tests are pinned by digest.
- CLAUDE.md applies (Conventional Commits, `make check` green, docs updated in the same change).

## Review Focus

1. **Moving tag points to a revision already deployed** (e.g. timer fires twice): nothing happens, no restart. Test in Task 2: `agent-test` case "no-op".
2. **New image whose app never becomes healthy**: training rolls back, prod untouched, revision recorded as failed, next run skips it without pulling/restarting. Test in Task 2: case "bad revision".
3. **Baked `compose.yaml`/`Caddyfile` invalid** in a new image: agent refuses to install them (keeps the current files), records the revision as failed, deploys nothing. Test in Task 2: case "bad config".
4. **Registry unreachable / pull fails**: agent logs one error and exits non-zero; nothing changed; next tick retries (a pull failure is NOT recorded as a failed revision). Test in Task 2: case "registry down".
5. **Image without the revision label** (built outside CI): refused, nothing deployed. Test in Task 2: case "no label".

---

### Task 1: Image carries its revision and deploy config; CI only builds

**Files:** Modify `Dockerfile`, `.github/workflows/ci.yml`, `deploy/deploy.sh`, `deploy/env.example`; Create `docs/decisions/0016-pull-deploy.md`.

- [ ] Step 1: `Dockerfile`: `ARG REVISION=unknown` in the final stage; `LABEL org.opencontainers.image.revision=$REVISION org.opencontainers.image.source=https://github.com/fatpat/acroworldtour.com`; `COPY compose.yaml Caddyfile /usr/share/awt/deploy/`. `make docker` passes `--build-arg REVISION=$(git rev-parse HEAD)`.
- [ ] Step 2: `ci.yml`: `build` job pushes tags `:<github.sha>` and `:rewrite-go` (replace `:latest`), passes `build-args: REVISION=${{ github.sha }}`; delete the `deploy` job; rewrite the header comment (no secrets needed; the VPS agent deploys).
- [ ] Step 3: `deploy.sh`: remove the SSH forced-command block, `sync` and the `docker login` path; keep `deploy.sh training|prod <tag>` behaviour identical (pull, up, health check, Caddy validate+reload+check, rollback). `env.example`: add `AWT_CHANNEL=rewrite-go`.
- [ ] Step 4: Decision record 0016 (pull deploy; trust model: whoever can push the moving tag deploys; signature verification deferred).
- [ ] Step 5: Verify: `make docker` then `docker inspect -f '{{index .Config.Labels "org.opencontainers.image.revision"}}' awt:dev` prints the HEAD sha and `docker run --rm --entrypoint ls awt:dev /usr/share/awt/deploy` lists both files; `sh -n deploy/deploy.sh`; YAML parses. Commit `ci: build-only pipeline; image carries revision and deploy config`.

### Task 2: The agent, its systemd units and its test

**Files:** Create `deploy/agent.sh`, `deploy/awt-agent.service`, `deploy/awt-agent.timer`, `scripts/agent-test.sh`; Modify `Makefile` (`agent-test` target, also listed in `.PHONY`; not part of `make check`), `docs/ops.md`.

**Interfaces — Consumes:** Task 1 image label/paths and `deploy.sh training|prod <tag>`.

- [ ] Step 1: Write `scripts/agent-test.sh` first (pattern: `scripts/backup-restore-test.sh`): start a local registry (`registry:2` pinned by digest) on a random port; build two images from the repo (good: normal; bad: `HEALTHCHECK` forced to fail via a build arg or a tiny overlay Dockerfile) and an image without the label and one with an invalid Caddyfile; set up a temp `/opt/awt`-like dir (env files, `caddy/`, `state/`) and run `agent.sh` with `AWT_ROOT=<tmp>`, `AWT_IMAGE=localhost:<port>/awt`, `AWT_CHANNEL=test`, short `HEALTH_TIMEOUT`. Cases (Review Focus 1–5) plus "happy path": retag channel → agent deploys training then prod (both tags = rev, files installed). Each case asserts tags in `.env`, files installed or untouched, `state/failed-revisions` content, and exit code. Run → FAIL (no agent yet).
- [ ] Step 2: Implement `deploy/agent.sh` (POSIX sh, `set -eu`, `AWT_ROOT` default `/opt/awt`): lock; `docker pull "$AWT_IMAGE:$AWT_CHANNEL"` (failure → log, exit 1, nothing recorded); revision = label `org.opencontainers.image.revision`, must match `^[0-9a-f]{40}$` else refuse (exit 1); if both `AWT_TRAINING_TAG` and `AWT_PROD_TAG` equal the revision → exit 0 silently; if revision in `state/failed-revisions` → log `skip` and exit 0; copy `/usr/share/awt/deploy/{compose.yaml,Caddyfile}` out of the image (`docker create` + `docker cp` into `state/staging-<rev>/`, then `docker rm`); validate: `docker compose -f staging/compose.yaml --env-file .env config -q` and `docker run --rm -v staging:/c caddy:<pinned> caddy validate --config /c/Caddyfile --adapter caddyfile` with the Caddy env vars it needs; on failure record + exit 1; install atomically (copy to `.new`, `mv`); then `deploy.sh training <rev>` → on failure record + exit 1; then `deploy.sh prod <rev>` → on failure record + exit 1; log `deployed <rev>`. Tags in deploy.sh use the full 40-char revision (deploy.sh already accepts 7–40 hex).
- [ ] Step 3: systemd: `awt-agent.service` (`Type=oneshot`, `User=deploy`, `ExecStart=/opt/awt/deploy/agent.sh`, `Nice=10`), `awt-agent.timer` (as Global Constraints, `WantedBy=timers.target`).
- [ ] Step 4: Run `make agent-test` → PASS; `sh -n` on all scripts.
- [ ] Step 5: `docs/ops.md`: replace the SSH deploy key / forced command / GitHub secrets / sync text with: install `agent.sh`, `deploy.sh` and the two units under `/opt/awt/deploy` and `/etc/systemd/system`, `systemctl enable --now awt-agent.timer`; how to watch (`journalctl -u awt-agent -f`), how to retry a failed revision, how to pause (`systemctl stop awt-agent.timer`), how to roll back manually (`deploy.sh prod <older sha>` + stop the timer); SSH stays for admins only (deploy user no longer needs SSH access: remove its key). Commit `feat(ops): pull-based deploy agent`.
