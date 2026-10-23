# 0016. Pull-based deploy: an agent on the VPS follows a moving image tag

Status: accepted
Date: 2026-10-23

## Context

Until now CI deployed by SSH: the `deploy` job of `.github/workflows/ci.yml` held a restricted deploy key (`VPS_*` secrets), synced `compose.yaml` and `Caddyfile` through a forced command (`deploy.sh sync`) and ran `deploy.sh training|prod <sha>`. This kept SSH open to the internet for GitHub-hosted runners (no fixed addresses, see `rulings-2026-10.md`), put a root-equivalent key (the `deploy` user is in the `docker` group) in GitHub secrets, and handed the run's registry token to the VPS.

The owner decided on 2026-10-23: a pull-based agent; both environments updated automatically with the same image (training first, prod only if training is healthy); results in the server logs only; the repository and the GHCR package are public.

## Decision

- CI only builds and pushes `ghcr.io/<owner>/awt:<sha>` and the moving tag `:rewrite-go` (named after the deploy branch). The image carries its commit in the OCI label `org.opencontainers.image.revision` and the deploy configuration (`compose.yaml`, `Caddyfile`) under `/usr/share/awt/deploy/`, so code and configuration ship as one artifact.
- On the VPS a systemd timer runs `deploy/agent.sh` every 60 s as the `deploy` user. It pulls the moving tag anonymously (`AWT_CHANNEL` in `/opt/awt/.env`, default `rewrite-go`), reads the revision label (40 lowercase hex characters, otherwise refused), validates the baked `compose.yaml` and `Caddyfile`, installs them, then runs the existing `deploy.sh training <rev>` and, only if it succeeded, `deploy.sh prod <rev>` (health check and rollback unchanged).
- A revision that fails validation or either deploy is recorded in `/opt/awt/state/failed-revisions` and never retried automatically; deleting its line retries it. A failed pull is not recorded: the next tick retries.
- Results go to journald (`journalctl -u awt-agent`). No notification channel.
- The SSH forced-command mode and `sync` are removed from `deploy.sh`; the `deploy` user has no SSH key. Admin SSH is unchanged.

## Trust model

Whoever can push the moving tag to the GHCR package deploys to prod: in practice, a workflow run on the deploy branch (the `GITHUB_TOKEN` with `packages: write`) or a maintainer of the repository. This is the same trust boundary as before (whoever could push to the deploy branch deployed), minus the SSH key and the registry token on the VPS. Image signature verification (for example cosign keyless signatures checked by the agent) is deferred: it would bind deploys to the CI workflow identity instead of package write access.

## Consequences

- No inbound connection is needed for deploys; SSH only serves admins and can be restricted further.
- Deploy latency is up to one poll interval (60 s) after the image is pushed; CI shows "image published", not "deployed". Deploy outcomes are only visible in the VPS logs.
- An image built outside CI (no revision label) is never deployed by the agent.
- `deploy.sh` and `agent.sh` are not shipped by the image: they are copied by hand when they change, like `weekly-restore-test.sh`.
- Supersedes the "deploy SSH" ruling in `rulings-2026-10.md`.
