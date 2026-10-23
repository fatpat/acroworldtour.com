# Operations

One OVHcloud VPS runs Docker Compose: `caddy`, `awt-prod`, `awt-training`. Cloudflare sits in front. Each awt container runs `litestream replicate -exec "awt serve"` and replicates its SQLite database to OVHcloud Object Storage (`litestream/<env>/` in the bucket). Design: `specifications/08-rewrite-recommendations.md` section 6. Image decision: `docs/decisions/0013-runtime-image.md`. Deploy decision: `docs/decisions/0016-pull-deploy.md`.

Layout on the VPS: `/opt/awt/{compose.yaml,caddy/Caddyfile,certs/,state/,.env*}`, `/opt/awt/deploy/{agent.sh,deploy.sh,weekly-restore-test.sh}` and `/etc/systemd/system/awt-agent.{service,timer}`.

Files: `Dockerfile`, `litestream.yml`, `compose.yaml`, `Caddyfile`, `deploy/` (entrypoint, deploy agent and its systemd units, deploy and restore scripts, env examples), `.github/workflows/ci.yml` (job `build`, run after `check` on a push to the deploy branch: it publishes the image, it does not deploy).

How a release reaches the VPS: CI pushes `ghcr.io/<owner>/awt:<sha>` and the moving tag `:rewrite-go` (public package; the image carries its commit in the `org.opencontainers.image.revision` label and `compose.yaml` + `Caddyfile` under `/usr/share/awt/deploy/`). On the VPS, `awt-agent.timer` runs `deploy/agent.sh` every 60 s: it pulls the moving tag and, when it carries a new revision, installs the baked config and deploys training, then prod (section 5). Nothing connects to the VPS to deploy.

## 1. Provision the VPS

1. Order an OVHcloud VPS (Debian 12 or Ubuntu 24.04 LTS). Add your SSH public key at order time.
2. Create a non-root deploy user `deploy` in the `docker` group (see the note on root equivalence below), and an admin user for yourself. Harden SSH in `/etc/ssh/sshd_config.d/10-hardening.conf`, then `systemctl reload ssh`:
   ```
   Port 2222                      # any non-default port
   PasswordAuthentication no
   PermitRootLogin no
   AllowUsers admin
   ```
3. SSH is for admins only: deploys do not use it (the agent pulls). Protect it with key-only authentication, the non-default port and `fail2ban` (`apt install fail2ban`, enable the `sshd` jail with the custom port). Allow your admin IPs only if you have fixed ones; otherwise accept the exposure.
4. **The deploy user has no SSH access.** Membership of the `docker` group is root-equivalent: anyone who can run `docker` as `deploy` can mount the host filesystem. `deploy` only runs the agent (systemd) and the restore-test cron: give it no `authorized_keys` (remove any key left from the former SSH deploy, `/home/deploy/.ssh/authorized_keys`) and keep it out of `AllowUsers`. Keep `/opt/awt/deploy/` owned by root (or by an admin account, not `deploy`) and not writable by `deploy`. This only prevents accidental changes: `deploy` is root-equivalent through Docker, so whoever controls that account (or the images it runs) can change anything on the host anyway. The real boundary is who can push the moving tag (decision 0016).
5. Install Docker Engine and the compose plugin from Docker's apt repository. Docker Compose 2.22 or later is required (`deploy.sh` uses `docker compose pull --policy missing`); check with `docker compose version`.
6. Enable automatic security updates: `apt install unattended-upgrades` and `dpkg-reconfigure -plow unattended-upgrades`.
7. Create the layout: `install -d -o deploy -m 750 /opt/awt /opt/awt/certs /opt/awt/state`, `install -d -o deploy -m 755 /opt/awt/caddy` (the agent installs the Caddyfile into it) and `install -d -o root -m 755 /opt/awt/deploy`. Copy `deploy/agent.sh`, `deploy/deploy.sh` and `deploy/weekly-restore-test.sh` to `/opt/awt/deploy/` (as the admin user, `chmod 755`, owned by root), `compose.yaml` to `/opt/awt/` and `Caddyfile` to `/opt/awt/caddy/` (a directory, mounted into Caddy; a single-file mount would go stale when the agent replaces the file). From then on the agent installs `compose.yaml` and `Caddyfile` from each new image; the three scripts change rarely and are copied by hand again when they change in the repository.
8. Firewall: allow SSH (your port), allow 80 and 443 from Cloudflare ranges only, deny everything else inbound. Docker bypasses ufw for published ports, so filter in the `DOCKER-USER` chain (or use the OVHcloud network firewall in front of the VPS). Cloudflare ranges: https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6. One allow rule per CIDR on tcp/80, tcp/443 (and udp/443 for HTTP/3), then a final deny. The Caddyfile also aborts any request whose peer is not a Cloudflare address (defence in depth). Refresh the CIDR list in the firewall and in the `Caddyfile` when Cloudflare changes it (check quarterly).
9. **TLS strategy: Cloudflare Origin CA certificate, SSL mode Full (strict).** In Cloudflare, create an Origin CA certificate covering the prod and training hosts (or `*.example.org`), validity up to 15 years. Save it as `/opt/awt/certs/origin.pem` and the private key as `/opt/awt/certs/origin.key` (`chmod 700 certs`, key `600`; if Caddy cannot read the key because Docker runs with user namespaces or rootless, use `640` with a group Caddy shares, or `644` inside the 700 directory). Paths can be changed with `AWT_TLS_CERT` and `AWT_TLS_KEY` in the Caddy environment. Caddy loads these files instead of using ACME, so no certificate is requested from Let's Encrypt, and nothing but Cloudflare can complete a trusted TLS handshake with the origin. The certificate is not trusted by browsers; the origin must never be exposed without Cloudflare. Replace it before it expires: write the new files and run `docker compose restart caddy`.
10. Cloudflare: proxied DNS records for the prod and training hosts. Do not cache `/live/*` (add a cache rule: bypass). Cloudflare closes idle connections after about 100 s; the live stream sends a heartbeat about every 30 s (spec 6.4) and Caddy does not buffer `/live/*`.

## 2. Secrets and configuration

Nothing secret is committed. On the VPS, in `/opt/awt` (mode 600, owner = deploy user):

| File | From | Holds |
|---|---|---|
| `.env` | `deploy/env.example` | hosts, image name, moving tag the agent follows (`AWT_CHANNEL`, default `rewrite-go`), deployed image tags (rewritten by `deploy.sh`; `none` = nothing deployed yet, there is no `latest` default) |
| `.env.prod` | `deploy/env.prod.example` | `AWT_*` for prod: base URL (required), S3 keys, Cloudflare token, trusted proxies |
| `.env.training` | `deploy/env.training.example` | same for training, with its own base URL and bucket |

There is no session secret: sessions are random tokens stored in the database. `AWT_ENV` is set per service in `compose.yaml` (`prod`, `training`); the app refuses to start outside dev without `AWT_BASE_URL`. Set `AWT_IMAGE` in `.env` to `ghcr.io/<lowercase-owner>/awt`. Create the Object Storage bucket(s) and an S3 user with read/write access to them. `LITESTREAM_S3_REGION` is the OVH region (for example `gra`). Leave `AWT_S3_BUCKET` empty to run without replication (not for prod).

**Manager MFA key.** TOTP secrets are stored encrypted with `AWT_MFA_KEY` (base64, 32 bytes). Generate it once per environment with `openssl rand -base64 32` and put it in `.env.prod` / `.env.training` (different values). The app refuses to start outside dev if any TOTP secret exists and the key is missing; if the key is lost or changed, enrolled TOTP secrets become unusable and each affected manager must be reset by an admin and re-enrol. Back it up with the other secrets. In dev a fixed key is used and a warning is logged.

**WebAuthn RP ID.** `AWT_WEBAUTHN_RP_ID` defaults to the host of `AWT_BASE_URL` (`localhost` in dev). Set it only to share passkeys across subdomains (a parent domain of the site host). Changing it later invalidates every registered passkey, so decide before managers enrol. The accepted WebAuthn origin is the origin of `AWT_BASE_URL` (in dev without it, `http://localhost:<port>` and `http://127.0.0.1:<port>` from `AWT_ADDR`), so security keys only work when the manager is opened at that address.

Deploys need no GitHub secret and no registry credential on the VPS: the repository and the GHCR package are public, so the agent, `deploy.sh` and the restore test pull anonymously. If the package is ever made private, run `docker login ghcr.io` once as `deploy` with a read-only personal access token.

## 3. First deploy

1. Provision and configure as above, then install the agent (as root):
   ```sh
   install -m 644 awt-agent.service awt-agent.timer /etc/systemd/system/   # from deploy/ in the repository
   systemctl daemon-reload
   systemctl enable --now awt-agent.timer
   ```
   Push to the deploy branch (`rewrite-go`, set in the `if:` of the `build` job in `.github/workflows/ci.yml`; the moving tag has the same name). After `check` passes, `build` publishes the image; within a minute the agent installs its `compose.yaml` and `Caddyfile`, deploys training (health checked), then prod. Follow it with `journalctl -u awt-agent -f`. The first run starts empty databases (migrations run at `awt serve` startup).
2. Check: `docker compose ps` shows `healthy` for `awt-prod` and `awt-training`; `docker compose logs awt-prod` shows `listening`; the public URL answers through Cloudflare; the bucket gets a `litestream/prod/` prefix within a minute.
3. Create the first admin: `docker compose exec awt-prod awt user add --email you@example.org --name "You" --role admin` (the password is read from stdin or prompted).

## 4. First import (legacy data)

Production data lives in the MongoDB export (`mongo.backup.*.tar.gz`, never committed). Import on the VPS into an empty database:

```sh
scp -P 2222 mongo.backup.YYYYMMDD-HHMM.tar.gz admin@<vps-host>:/tmp/
ssh -p 2222 admin@<vps-host>     # your admin account; deploy has no SSH access
cd /opt/awt
docker compose cp /tmp/mongo.backup.*.tar.gz awt-training:/tmp/export.tar.gz
docker compose exec awt-training awt import --from /tmp/export.tar.gz
```

Run it on training first and check the result in the browser. Then repeat on prod (`awt-prod`) before opening the site. The import writes through the normal database connection, so Litestream replicates it. Delete the archive from the VPS and the container (`rm`) afterwards.

## 5. Routine deploys and rollback

Every push to the deploy branch that passes `check` publishes a new image; the agent deploys it automatically, training then prod, with the same revision. One pass of `deploy/agent.sh` (systemd timer, every 60 s, one run at a time through `flock` on `/run/lock/awt-agent.lock`):

1. Pulls `$AWT_IMAGE:$AWT_CHANNEL`. A failed pull (registry down, network) logs one error and changes nothing; the next tick retries.
2. Reads the commit from the `org.opencontainers.image.revision` label. An image without a 40-character commit id (built outside CI) is refused and nothing is deployed.
3. If training and prod already run that revision, stops silently (no restart).
4. If the revision is listed in `/opt/awt/state/failed-revisions`, logs `skip <rev>: failed before` and stops.
5. Copies `compose.yaml` and `Caddyfile` out of the image and validates them (`docker compose config` with `.env`; `caddy validate` with the Caddy image of the new `compose.yaml`, the hosts from `.env` and `/opt/awt/certs`). Invalid: the revision is recorded as failed and the installed files stay as they are.
6. Saves the current files and deployed tags (`/opt/awt/state/prev/`), installs the two files (write then rename), tags the pulled image as `:<rev>` locally (so `deploy.sh` does not pull again), runs `deploy.sh training <rev>`, then, only if training succeeded, `deploy.sh prod <rev>`. A failure of either (app not healthy, or Caddy validate/reload/health check failing after a healthy app) records the revision as failed; a failed pull inside `deploy.sh` (exit 3) is not recorded and retried next tick. In every failure case after the install (including a failed command, `INT`/`TERM` or the systemd start timeout) the agent puts the previous `compose.yaml` and `Caddyfile` back and redeploys the previous training revision (and the previous prod revision if prod was attempted), so both apps and Caddy end on the previous revision with its own config. A restore that itself fails logs `RESTORE FAILED`: check by hand. If a run is killed outright (`SIGKILL`, power loss) between install and the end, the next run notices it (`previous run did not finish`; `state/installed-ok` is missing) and keeps the saved `state/prev/` instead of saving the half-installed files over it.
7. After a successful deploy, removes local images of `$AWT_IMAGE` older than `AWT_PRUNE_DAYS` days (environment of the service, digits only, default 30) except the new revision, the previous training and prod revisions and the moving tag (`$AWT_CHANNEL`), then this project's dangling images as old. Pruning never fails a run.

`deploy.sh <env> <tag>` waits up to 120 s for the container health check, then requests `/healthz` inside the stack (directly on the app container, then through Caddy on its loopback-only listener `127.0.0.1:8081` in the `caddy` container, which is never published). Caddy's config is validated and reloaded on every deploy; a validation or reload failure fails the deploy (no silent success). If the new app container is not healthy it restarts the previous tag and exits non-zero. On the very first deploy there is no previous tag and nothing to roll back to.

Operating the agent (as an admin, with `sudo`):

- **Watch:** `journalctl -u awt-agent -f` (outcomes are only in the VPS logs; CI only reports the image as published). `systemctl list-timers awt-agent.timer` shows the next run.
- **Retry a failed revision:** delete its line from `/opt/awt/state/failed-revisions`; the next tick deploys it again. Pushing a new commit also deploys normally (only the failed revision is skipped).
- **Pause deploys:** `systemctl stop awt-agent.timer` (`systemctl start awt-agent.timer` to resume). A run in progress finishes.
- **Manual rollback** (for example a bad release that is healthy but wrong): first `systemctl stop awt-agent.timer`, otherwise the agent redeploys the moving tag within a minute; then `sudo -u deploy /opt/awt/deploy/deploy.sh prod <previous-sha>` (and `training` if needed). Tags are the commit SHAs in the GHCR package list. Restart the timer once a fixed commit is published (or record the bad revision in `failed-revisions` first). Migrations only move forward and must stay compatible with the previous release for one version; if a migration itself is the problem, restore the database (next section).
- **Updating the agent or `deploy.sh`:** copy the new script to `/opt/awt/deploy/` by hand (root-owned, `chmod 755`); they are not shipped by the image.

**Local test:** `make agent-test` runs the agent against a local registry and a temporary `/opt/awt` through the real `deploy.sh` and `compose.yaml`: happy path, no-op, lock held, bad revision (rollback, previous config restored, recorded, skipped next run), invalid baked Caddyfile or compose.yaml, image with an `unknown` or no revision label, registry down then back, pull failing inside `deploy.sh` (not recorded, retried), `:<rev>` missing from the registry but present locally, prod failing after a healthy training (both restored). It needs Docker and takes a few minutes; it is not part of `make check`.

## 6. Backups and restore

Litestream replicates continuously (about 1 s lag), snapshots every 6 h, keeps 30 days (`litestream.yml`).

**Automatic restore on an empty volume.** The entrypoint runs `litestream restore -if-db-not-exists -if-replica-exists` before starting. To rebuild a lost or corrupted environment:

```sh
cd /opt/awt
docker compose stop awt-prod
docker compose rm -f awt-prod
docker volume rm awt_prod_data
docker compose up -d awt-prod          # restores from the replica, then serves
```

**Point in time restore** to a scratch file (does not touch prod):

```sh
docker compose run --rm --no-deps --entrypoint sh awt-prod -c \
  'litestream restore -config /etc/litestream.yml -timestamp 2026-10-18T12:00:00Z -o /tmp/pit.db /data/awt.db && ls -l /tmp/pit.db'
```

To make a point in time restore live, restore to `/data/awt.db` on an emptied volume (same command with `-o /data/awt.db` inside a container mounting the volume), then start the service.

**Local drill:** `make backup-restore-test` runs the whole cycle against a local MinIO in a few seconds (writes data, deletes the volume, starts again, checks the data is back).

**Weekly restore test (cron on the VPS).** `deploy/weekly-restore-test.sh` restores the latest prod replica into a throwaway container and runs `PRAGMA integrity_check`, requires at least one row in `users`, and opens it with `awt migrate`; it fails loudly on any of these. Install it in the deploy user's crontab (`sudo crontab -u deploy -e`):

```
MAILTO=you@example.org
RESTORE_TEST_PING_URL=https://hc-ping.com/<uuid>
17 4 * * 1  /opt/awt/deploy/weekly-restore-test.sh >> /var/log/awt-restore-test.log 2>&1
```

A failure exits non-zero (cron mails the output) and pings `$RESTORE_TEST_PING_URL/fail`; success pings the URL, so a missed run also alerts when the check is configured with a 8-day period (healthchecks.io style service). Create `/var/log/awt-restore-test.log` writable by the deploy user.

## 7. Monitoring and capacity

- Uptime: point UptimeRobot or Better Stack at `https://<prod-host>/healthz`.
- Logs: `docker compose logs -f awt-prod` (JSON in prod). Docker keeps them under `/var/lib/docker`; set `log-driver` options in `/etc/docker/daemon.json` (`max-size` 10m, `max-file` 5).
- Before the first live competition, run the k6 load test against training with Cloudflare in front.

## 7b. Lost second factor (manager MFA)

A manager who lost their authenticator app or security key and has no recovery code left cannot log in. Reset them; they then log in with their password only and set a second factor up again from `/manager/account`.

- **Another admin is available:** open `/manager/users`, choose the user (Manage), press "Remove second factor", then "Confirm reset". This removes the app, the keys and the recovery codes, clears any MFA lockout and closes all the user's sessions. An admin cannot reset their own account here: use the account page.
- **The only admin is locked out** (or no admin can log in): reset from the server, with the account's email (the command needs no password):
  ```sh
  docker compose exec awt-prod awt user reset-mfa --email you@example.org
  ```
- Every reset logs `event=mfa.reset_by_admin` with `user_id`, `actor` (the admin's user id, or `cli`) and `ip` (empty from the CLI). Hand the new password over by another channel if you also reset it (`awt user passwd`).
- **Back up `AWT_MFA_KEY` with the other secrets.** TOTP secrets are encrypted with it: if it is lost or changed, every enrolled authenticator app stops working at once (security keys and recovery codes are unaffected) and each such manager needs a reset as above. Never rotate it casually.
- **Restarting after losing `AWT_MFA_KEY`.** The app refuses to start outside dev while TOTP secrets exist and no key is set, so pick one:
  1. Set a **new** key (`openssl rand -base64 32`) in the environment file and start. The old TOTP secrets can no longer be opened: those managers see "authenticator codes unavailable" at login and sign in with a security key or a recovery code, or are reset by an admin. Once in, they remove the dead app on the account page and enrol again.
  2. Or, **before** restarting, reset every TOTP user from the server with `awt user reset-mfa --email ...` (it works without the key); then the app starts without a key (authenticator apps stay unavailable until one is set, and a startup warning says so).

## 8. Developer setup: browser tests

`make e2e` drives the real binary with chromium (playwright-go, test-only dependency). Install the driver and the browser once:

```sh
make e2e-deps
```

`scripts/install-playwright.sh` unpacks the Playwright 1.57.0 driver from a pinned PyPI wheel (sha256 checked) into `~/.cache/ms-playwright-go/1.57.0`, because the CDN playwright-go v0.5700.1 uses is gone, then installs chromium into `~/.cache/ms-playwright` with the pinned playwright-go CLI. It needs `curl`, `python3` and network access, not root. CI runs it before `make check` and caches both directories. Failures leave traces and screenshots in `e2e/.artifacts/` (git-ignored).
