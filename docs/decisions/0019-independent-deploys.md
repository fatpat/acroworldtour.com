# 0019. Training and prod deployed independently

Status: accepted
Date: 2026-10-23
Supersedes: the deploy order of decision 0016 ("training first, prod only if training is healthy")

## Context

Decision 0016 had the deploy agent run each new revision on training first and on prod only if training was healthy, so training served as a gate. A revision that failed on either environment was recorded once, for both, and the whole run was rolled back.

The owner decided on 2026-10-23, verbatim: "remove the rule on deployment to training first then to prod. Deploy on both at the same time."

## Decision

- Each new revision is deployed to training and to prod in the same agent run, **independently**: neither environment's result gates or undoes the other. The two `deploy.sh` calls still run one after the other (training, then prod) within that run. They share the Caddy validation and reload and the installed `compose.yaml`/`Caddyfile`, and running them sequentially keeps one Caddy reload at a time and one readable log. Running them in parallel would save under two minutes per release, so it is not worth the shared-reload races.
- Failures are recorded per environment: `state/failed-revisions.training` and `state/failed-revisions.prod`. A revision that failed on one environment is never retried there (deleting its line retries it) and is still deployed to the other. A failed pull inside `deploy.sh` (exit 3) is not recorded and is retried next run.
- Rollback is per environment: a failed environment goes back to its previous revision, the other keeps the new one.
- The baked `compose.yaml` and `Caddyfile` are shared:
  - If they fail validation, no environment gets the revision, and it is recorded as failed on both.
  - If at least one environment runs the revision, its files stay installed and a failed environment is rolled back to its previous image under the previous `compose.yaml` (`state/prev/compose.yaml`, given to `deploy.sh` as `AWT_COMPOSE_FILE`), so it runs exactly as before. The `Caddyfile` is shared and stays the new one: as a technical constraint of the single shared proxy, a `Caddyfile` change must keep serving the previous app version. Until a new revision arrives, the failed environment stays on its previous image under that previous definition; a later deploy of it uses the installed files.
  - If no environment took it, the previous files are put back before the failed environments are rolled back.
- The no-op check and the `state/installed-ok` repair work per environment: a run has nothing to do when each environment is on the revision, skipped or failed there before. The marker is repaired when at least one environment runs the revision.
- While a training refresh is incomplete (`state/sync-training.incomplete`, decision 0018), training is skipped and prod proceeds. This is the general rule applied to an environment that cannot be deployed, not a special policy.
- Migration: an existing `state/failed-revisions` (one list for both environments) is appended to both new lists once and kept as `state/failed-revisions.migrated`: its revisions count as failed on both.

## Consequences

- A revision may reach prod without running on training first, and training no longer protects prod from a bad release. The health check and per-environment rollback in `deploy.sh` remain.
- Training and prod may run different revisions for a while (one failed, the other did not); `journalctl -u awt-agent` and the two failed lists show which.
- After a partial success the failed environment runs its previous image under its previous `compose.yaml` but behind the new `Caddyfile` (the shared proxy, above).
- A failed pull is neither recorded nor rolled back: nothing changed on that environment.
