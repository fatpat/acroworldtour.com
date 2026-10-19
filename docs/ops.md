# Operations

One OVHcloud VPS runs Docker Compose: `caddy`, `awt-prod`, `awt-training`. Cloudflare sits in front. Each awt container runs `litestream replicate -exec "awt serve"` and replicates its SQLite database to OVHcloud Object Storage (`litestream/<env>/` in the bucket). Design: `specifications/08-rewrite-recommendations.md` section 6. Image decision: `docs/decisions/0013-runtime-image.md`.

Layout on the VPS: `/opt/awt/{compose.yaml,caddy/Caddyfile,certs/,.env*}` and `/opt/awt/deploy/{deploy.sh,weekly-restore-test.sh}`.

Files: `Dockerfile`, `litestream.yml`, `compose.yaml`, `Caddyfile`, `deploy/` (entrypoint, deploy and restore scripts, env examples), `.github/workflows/deploy.yml`.

## 1. Provision the VPS

1. Order an OVHcloud VPS (Debian 12 or Ubuntu 24.04 LTS). Add your SSH public key at order time.
2. Create a non-root deploy user `deploy` in the `docker` group (see the note on root equivalence below), and an admin user for yourself. Harden SSH in `/etc/ssh/sshd_config.d/10-hardening.conf`, then `systemctl reload ssh`:
   ```
   Port 2222                      # any non-default port; also set VPS_PORT in GitHub
   PasswordAuthentication no
   PermitRootLogin no
   AllowUsers admin deploy
   ```
3. SSH is reachable from the internet (GitHub-hosted runners have no fixed addresses), so it is protected by: key-only authentication, the non-default port, `fail2ban` (`apt install fail2ban`, enable the `sshd` jail with the custom port) and the restricted deploy key below. Allow your admin IPs only if you have fixed ones; otherwise accept the exposure.
4. **Deploy key restriction.** Membership of the `docker` group is root-equivalent: anyone who can run `docker` as `deploy` can mount the host filesystem. The CI key must therefore never get a shell. In `/home/deploy/.ssh/authorized_keys` use a forced command:
   ```
   restrict,command="/opt/awt/deploy/deploy.sh" ssh-ed25519 AAAA... awt-ci-deploy
   ```
   `restrict` disables the pty, port, agent and X11 forwarding. With `command=`, whatever the client asks for arrives in `SSH_ORIGINAL_COMMAND`, and `deploy.sh` accepts only `training <hex-tag>`, `prod <hex-tag>` (tag = lowercase hex commit id) or `sync` (a tar holding exactly `compose.yaml` and `Caddyfile`, regular files only). Any other input, including shell metacharacters, is refused before anything runs. A stolen CI key can therefore redeploy an image tag from the project's registry or replace the two config files, not run arbitrary commands. Keep `/opt/awt/deploy/` owned by root (or by an admin account, not `deploy`) and not writable by `deploy`, so the forced command cannot be rewritten through the key. Your own admin key is unrestricted.
5. Install Docker Engine and the compose plugin from Docker's apt repository.
6. Enable automatic security updates: `apt install unattended-upgrades` and `dpkg-reconfigure -plow unattended-upgrades`.
7. Create the layout: `install -d -o deploy -m 750 /opt/awt /opt/awt/certs`, `install -d -o deploy -m 755 /opt/awt/caddy` (deploys `sync` the Caddyfile into it) and `install -d -o root -m 755 /opt/awt/deploy`. Copy `deploy/deploy.sh` and `deploy/weekly-restore-test.sh` to `/opt/awt/deploy/` (as the admin user, `chmod 755`, owned by root), `compose.yaml` to `/opt/awt/` and `Caddyfile` to `/opt/awt/caddy/` (a directory, mounted into Caddy; a single-file mount would go stale when `sync` replaces the file). Deploys update `compose.yaml` and `Caddyfile` through `sync`; the two scripts change rarely and are copied by hand again when they change in the repository.
8. Firewall: allow SSH (your port), allow 80 and 443 from Cloudflare ranges only, deny everything else inbound. Docker bypasses ufw for published ports, so filter in the `DOCKER-USER` chain (or use the OVHcloud network firewall in front of the VPS). Cloudflare ranges: https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6. One allow rule per CIDR on tcp/80, tcp/443 (and udp/443 for HTTP/3), then a final deny. The Caddyfile also aborts any request whose peer is not a Cloudflare address (defence in depth). Refresh the CIDR list in the firewall and in the `Caddyfile` when Cloudflare changes it (check quarterly).
9. **TLS strategy: Cloudflare Origin CA certificate, SSL mode Full (strict).** In Cloudflare, create an Origin CA certificate covering the prod and training hosts (or `*.example.org`), validity up to 15 years. Save it as `/opt/awt/certs/origin.pem` and the private key as `/opt/awt/certs/origin.key` (`chmod 700 certs`, key `600`; if Caddy cannot read the key because Docker runs with user namespaces or rootless, use `640` with a group Caddy shares, or `644` inside the 700 directory). Paths can be changed with `AWT_TLS_CERT` and `AWT_TLS_KEY` in the Caddy environment. Caddy loads these files instead of using ACME, so no certificate is requested from Let's Encrypt, and nothing but Cloudflare can complete a trusted TLS handshake with the origin. The certificate is not trusted by browsers; the origin must never be exposed without Cloudflare. Replace it before it expires: write the new files and run `docker compose restart caddy`.
10. Cloudflare: proxied DNS records for the prod and training hosts. Do not cache `/live/*` (add a cache rule: bypass). Cloudflare closes idle connections after about 100 s; the live stream sends a heartbeat about every 30 s (spec 6.4) and Caddy does not buffer `/live/*`.

## 2. Secrets and configuration

Nothing secret is committed. On the VPS, in `/opt/awt` (mode 600, owner = deploy user):

| File | From | Holds |
|---|---|---|
| `.env` | `deploy/env.example` | hosts, image name, image tags (tags are rewritten by `deploy.sh`; `none` = nothing deployed yet, there is no `latest` default) |
| `.env.prod` | `deploy/env.prod.example` | `AWT_*` for prod: base URL (required), S3 keys, Cloudflare token, trusted proxies |
| `.env.training` | `deploy/env.training.example` | same for training, with its own base URL and bucket |

There is no session secret: sessions are random tokens stored in the database. `AWT_ENV` is set per service in `compose.yaml` (`prod`, `training`); the app refuses to start outside dev without `AWT_BASE_URL`. Set `AWT_IMAGE` in `.env` to `ghcr.io/<lowercase-owner>/awt`. Create the Object Storage bucket(s) and an S3 user with read/write access to them. `LITESTREAM_S3_REGION` is the OVH region (for example `gra`). Leave `AWT_S3_BUCKET` empty to run without replication (not for prod).

**Manager MFA key.** TOTP secrets are stored encrypted with `AWT_MFA_KEY` (base64, 32 bytes). Generate it once per environment with `openssl rand -base64 32` and put it in `.env.prod` / `.env.training` (different values). The app refuses to start outside dev if any TOTP secret exists and the key is missing; if the key is lost or changed, enrolled TOTP secrets become unusable and each affected manager must be reset by an admin and re-enrol. Back it up with the other secrets. In dev a fixed key is used and a warning is logged.

**WebAuthn RP ID.** `AWT_WEBAUTHN_RP_ID` defaults to the host of `AWT_BASE_URL` (`localhost` in dev). Set it only to share passkeys across subdomains (a parent domain of the site host). Changing it later invalidates every registered passkey, so decide before managers enrol.

GitHub repository secrets for the deploy workflow:

- `VPS_HOST`, `VPS_USER` (`deploy`), `VPS_PORT`: the VPS address, deploy user and SSH port.
- `VPS_SSH_KEY`: private key of a key pair used only for deploys (public half in `authorized_keys` with the forced command, section 1).
- `VPS_KNOWN_HOSTS`: `ssh-keyscan -p <port> -t ed25519 <host>`, checked against the fingerprint shown in the OVH console.

The workflow passes the run's `GITHUB_TOKEN` (valid for the run) on stdin to the forced command, which runs `docker login ghcr.io` with it. To pull outside a deploy (cron restore test, manual `compose up`), keep the last image already pulled, or make the package public, or run `docker login ghcr.io` once with a read-only personal access token.

## 3. First deploy

1. Provision and configure as above. Push to `main`; after CI the deploy workflow syncs `compose.yaml` and `Caddyfile`, deploys training (health checked), then deploys prod. The first run starts empty databases (migrations run at `awt serve` startup).
2. Check: `docker compose ps` shows `healthy` for `awt-prod` and `awt-training`; `docker compose logs awt-prod` shows `listening`; the public URL answers through Cloudflare; the bucket gets a `litestream/prod/` prefix within a minute.
3. Create the first admin: `docker compose exec awt-prod awt user add --email you@example.org --name "You" --role admin` (the password is read from stdin or prompted).

## 4. First import (legacy data)

Production data lives in the MongoDB export (`mongo.backup.*.tar.gz`, never committed). Import on the VPS into an empty database:

```sh
scp mongo.backup.YYYYMMDD-HHMM.tar.gz vps:/opt/awt/
ssh vps
cd /opt/awt
docker compose cp mongo.backup.*.tar.gz awt-training:/tmp/export.tar.gz
docker compose exec awt-training awt import --from /tmp/export.tar.gz
```

Run it on training first and check the result in the browser. Then repeat on prod (`awt-prod`) before opening the site. The import writes through the normal database connection, so Litestream replicates it. Delete the archive from the VPS and the container (`rm`) afterwards.

## 5. Routine deploys and rollback

Every push to `main` that passes CI deploys automatically: training, then prod. `deploy.sh <env> <tag>` waits up to 120 s for the container health check, then requests `/healthz` inside the stack (directly on the app container, then through Caddy on its loopback-only listener `127.0.0.1:8081` in the `caddy` container, which is never published). Caddy's config is validated and reloaded on every deploy; a validation or reload failure fails the deploy (no silent success). If the new app container is not healthy it restarts the previous tag and exits non-zero, which fails the workflow. On the very first deploy there is no previous tag and nothing to roll back to.

Manual rollback (for example a bad release that is healthy but wrong): `/opt/awt/deploy/deploy.sh prod <previous-sha>` (as `deploy` or an admin user; run `docker login ghcr.io` first if the image is not already local). Tags are the commit SHAs in the GHCR package list. Migrations only move forward and must stay compatible with the previous release for one version; if a migration itself is the problem, restore the database (next section).

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

**Weekly restore test (cron on the VPS).** `deploy/weekly-restore-test.sh` restores the latest prod replica into a throwaway container and runs `PRAGMA integrity_check`, requires at least one row in `users`, and opens it with `awt migrate`; it fails loudly on any of these. Install as the deploy user (`crontab -e`):

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

## 8. Developer setup: browser tests

`make e2e` drives the real binary with chromium (playwright-go, test-only dependency). Install the driver and the browser once:

```sh
make e2e-deps
```

`scripts/install-playwright.sh` unpacks the Playwright 1.57.0 driver from a pinned PyPI wheel (sha256 checked) into `~/.cache/ms-playwright-go/1.57.0`, because the CDN playwright-go v0.5700.1 uses is gone, then installs chromium into `~/.cache/ms-playwright` with the pinned playwright-go CLI. It needs `curl`, `python3` and network access, not root. CI runs it before `make check` and caches both directories. Failures leave traces and screenshots in `e2e/.artifacts/` (git-ignored).
