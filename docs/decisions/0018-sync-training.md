# 0018. Training refreshed from prod by hand, wiped first, MFA reset

Status: accepted
Date: 2026-10-23

## Context

Training runs next to prod on the same VPS (spec 08 §6, decision 0016) with its own database, volume and bucket prefix (decision 0017). Trainees learn the software there without breaking prod, so its data drifts away from prod's and fills with exercises. The owner asked for a way to bring it back to prod's state from time to time.

The owner decided on 2026-10-23:

- the refresh is **manual**: an admin runs a command on the VPS when needed, not a schedule;
- it is a **full clone**: database and uploaded files;
- training is **wiped first**: nothing of it remains (trainee accounts, its data, its files, its Litestream replica);
- **second factors are reset on training**: every session, pending login, authenticator app, security key, recovery code and MFA lockout is removed from the copy; users log in to training with their prod password.

## Decision

- `deploy/sync-training.sh`, installed on the VPS by `init-vps.sh`, run as root from `/opt/awt` (`docs/ops.md` section 6b). It asks to type `training` (or takes `--yes`; without a terminal `--yes` is required).
- It refuses before changing anything when a setting is missing, when either environment does not keep its uploads in S3, when prod is not running, when a running container does not use its env file's endpoint, bucket and prefix (`docker inspect`; the file was edited, the container not recreated), or when prod and training share a bucket (compared by name without case, whatever the endpoint) with prefixes that are equal or contain one another: wiping training could then delete prod's objects. It also refuses a prod database with migrations the training image does not know (newer tables could hold auth state the cleanup misses); a store test makes every table of the schema classified as cloned or reset, so a new auth table cannot be forgotten.
- The deploy agent is paused for the whole run (timer stopped, its lock held); the timer's previous state is restored on any exit.
- Prod is only read: a `VACUUM INTO` snapshot from a read-only connection while prod serves. The snapshot must pass `PRAGMA integrity_check`; it is migrated and cleaned (`store.ResetTrainingAuth`, one transaction) before training is touched.
- Then training's container, replica objects (`<prefix>litestream/`), files (`<prefix>files/`) and volume are deleted; the cleaned snapshot goes onto a new volume and prod's files are copied (server-side with the same endpoint and credentials, otherwise streamed with each side's credentials, content types kept). Training starts and Litestream begins a new replica from the copy. Training's host is purged from its Cloudflare zone when configured (by hostname, so a zone shared with prod keeps prod's cache).
- The S3 and database steps are Go (`internal/trainingsync`, hidden `awt sync-training` steps run in one-off containers of the app image), so the VPS needs no other tool.
- Every step can be re-run: a failed run is fixed by running the script again, which starts over.
- From the wipe until a successful end, `state/sync-training.incomplete` exists: `deploy.sh` refuses to deploy training, and the agent deploys new revisions to prod alone rather than holding prod back behind a stuck refresh (training is no test bed while it is broken).

## Consequences

- Trainee accounts and everything done on training are lost at each refresh; trainee accounts must be created again.
- Training holds a copy of prod's personal data and password hashes: its access must stay as restricted as prod's.
- Managers have no second factor on training after a refresh. Training's MFA key differs from prod's (init-vps.sh refuses equal keys), so copied TOTP secrets would not open there anyway, and passkeys are bound to the prod host.
- Training is down for the duration of the refresh (about a minute plus the file copy).
- While a refresh is incomplete, revisions reach prod without having run on training first.
