# Operations

One OVHcloud VPS runs Docker Compose: `caddy`, `awt-prod`, `awt-training`. Cloudflare sits in front. Each awt container runs `litestream replicate -exec "awt serve"` and replicates its SQLite database to OVHcloud Object Storage (`litestream/<env>/` in the bucket). Design: `specifications/08-rewrite-recommendations.md` section 6. Image decision: `docs/decisions/0013-runtime-image.md`.

Files: `Dockerfile`, `litestream.yml`, `compose.yaml`, `Caddyfile`, `deploy/` (entrypoint, deploy and restore scripts, env examples), `.github/workflows/deploy.yml`.

## 1. Provision the VPS

1. Order an OVHcloud VPS (Debian 12 or Ubuntu 24.04 LTS). Add your SSH public key at order time.
2. Create a non-root deploy user in the `docker` group, copy the key to it, then disable password and root SSH login in `/etc/ssh/sshd_config.d/10-hardening.conf`:
   ```
   PasswordAuthentication no
   PermitRootLogin no
   ```
   Then `systemctl reload ssh`.
3. Install Docker Engine and the compose plugin from Docker's apt repository.
4. Enable automatic security updates: `apt install unattended-upgrades` and `dpkg-reconfigure -plow unattended-upgrades`.
5. Create the deploy directory: `install -d -o <deploy-user> -m 750 /opt/awt`.
6. Firewall: allow SSH from your admin IPs, allow 80 and 443 from Cloudflare ranges only, deny everything else inbound. Docker bypasses ufw for published ports, so filter in the `DOCKER-USER` chain (or use the OVHcloud network firewall in front of the VPS). Cloudflare ranges: https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6. Example with nftables sets or the OVH firewall: one allow rule per CIDR on tcp/80, tcp/443 (and udp/443 for HTTP/3), then a final deny. The Caddyfile also aborts any request whose peer is not a Cloudflare address (defence in depth). Refresh the CIDR list in the firewall and the `Caddyfile` when Cloudflare changes it (check quarterly).
7. Cloudflare: proxied DNS records for the prod and training hosts, SSL mode **Full (strict)**. Caddy obtains its certificates with ACME (HTTP-01 works through Cloudflare; alternatively install a Cloudflare Origin CA certificate). Do not cache `/live/*` (add a cache rule: bypass). Cloudflare closes idle connections after about 100 s; the app sends an SSE heartbeat every 30 s and Caddy does not buffer `/live/*`.

## 2. Secrets and configuration

Nothing secret is committed. On the VPS, in `/opt/awt` (mode 600, owner = deploy user):

| File | From | Holds |
|---|---|---|
| `.env` | `deploy/env.example` | hosts, image name, image tags (tags are rewritten by `deploy.sh`) |
| `.env.prod` | `deploy/env.prod.example` | `AWT_*` for prod: session secret, S3 keys, Cloudflare token, trusted proxies |
| `.env.training` | `deploy/env.training.example` | same for training, with its own secret and bucket |

Generate session secrets with `openssl rand -hex 32`. Set `AWT_IMAGE` in `.env` to `ghcr.io/<lowercase-owner>/awt`. Create the Object Storage bucket(s) and an S3 user with read/write access to them. `LITESTREAM_S3_REGION` is the OVH region (for example `gra`). Leave `AWT_S3_BUCKET` empty to run without replication (not for prod).

GitHub repository secrets for the deploy workflow:

- `VPS_HOST`, `VPS_USER`: the VPS address and deploy user.
- `VPS_SSH_KEY`: private key of a key pair used only for deploys (public half in the deploy user's `authorized_keys`).
- `VPS_KNOWN_HOSTS`: `ssh-keyscan -t ed25519 <host>`, checked against the fingerprint shown in the OVH console.

The workflow logs the VPS into GHCR with the run's `GITHUB_TOKEN` (valid for the run). To pull outside a deploy (cron restore test, manual `compose up`), keep the last image already pulled, or make the package public, or run `docker login ghcr.io` once with a read-only personal access token.

## 3. First deploy

1. Provision and configure as above. Push to `main`; after CI the deploy workflow copies `compose.yaml`, `Caddyfile` and the scripts to `/opt/awt`, deploys training, smoke tests, then deploys prod. The first run starts empty databases (migrations run at `awt serve` startup).
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

Every push to `main` that passes CI deploys automatically: training, smoke test, prod. `deploy/deploy.sh <env> <tag>` waits up to 120 s for the container health check (`/healthz`). If the new container is not healthy it restarts the previous tag and exits non-zero, which fails the workflow.

Manual rollback (for example a bad release that is healthy but wrong): `cd /opt/awt && ./deploy.sh prod <previous-sha>`. Tags are the commit SHAs in the GHCR package list. Migrations only move forward and must stay compatible with the previous release for one version; if a migration itself is the problem, restore the database (next section).

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

**Weekly restore test (cron on the VPS).** `deploy/weekly-restore-test.sh` restores the latest prod replica into a throwaway container and opens it with `awt migrate`; it fails loudly on any restore or database error. Install as the deploy user (`crontab -e`):

```
MAILTO=you@example.org
RESTORE_TEST_PING_URL=https://hc-ping.com/<uuid>
17 4 * * 1  /opt/awt/weekly-restore-test.sh >> /var/log/awt-restore-test.log 2>&1
```

A failure exits non-zero (cron mails the output) and pings `$RESTORE_TEST_PING_URL/fail`; success pings the URL, so a missed run also alerts when the check is configured with a 8-day period (healthchecks.io style service). Create `/var/log/awt-restore-test.log` writable by the deploy user.

## 7. Monitoring and capacity

- Uptime: point UptimeRobot or Better Stack at `https://<prod-host>/healthz`.
- Logs: `docker compose logs -f awt-prod` (JSON in prod). Docker keeps them under `/var/lib/docker`; set `log-driver` options in `/etc/docker/daemon.json` (`max-size` 10m, `max-file` 5).
- Before the first live competition, run the k6 load test against training with Cloudflare in front.
