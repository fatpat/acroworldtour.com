# Operations

One OVHcloud VPS runs Docker Compose: `caddy`, `awt-prod`, `awt-training`. Cloudflare sits in front. Each awt container runs `litestream replicate -exec "awt serve"` and replicates its SQLite database to OVHcloud Object Storage (`<AWT_BUCKET_PREFIX>litestream/` in the bucket; uploaded files go under `<AWT_BUCKET_PREFIX>files/`, section 6). Design: `specifications/08-rewrite-recommendations.md` section 6. Image decision: `docs/decisions/0013-runtime-image.md`. Deploy decision: `docs/decisions/0016-pull-deploy.md`.

Layout on the VPS: `/opt/awt/{compose.yaml,caddy/Caddyfile,certs/,state/,.env*}`, `/opt/awt/deploy/{agent.sh,deploy.sh,weekly-restore-test.sh}` and `/etc/systemd/system/awt-agent.{service,timer}`.

Files: `Dockerfile`, `litestream.yml`, `compose.yaml`, `Caddyfile`, `deploy/` (entrypoint, deploy agent and its systemd units, deploy and restore scripts, env examples), `.github/workflows/ci.yml` (job `build`, run after `check` on a push to the deploy branch: it publishes the image, it does not deploy).

How a release reaches the VPS: CI pushes `ghcr.io/<owner>/awt:<sha>` and the moving tag `:rewrite-go` (public package; the image carries its commit in the `org.opencontainers.image.revision` label and `compose.yaml` + `Caddyfile` under `/usr/share/awt/deploy/`). On the VPS, `awt-agent.timer` runs `deploy/agent.sh` every 60 s: it pulls the moving tag and, when it carries a new revision, installs the baked config and deploys training, then prod (section 5). Nothing connects to the VPS to deploy.

## 0. New server in one command

`deploy/init-vps.sh` does sections 1 to 3 (and installs the agent of section 5) on a fresh **Debian 13 (trixie)** VPS, as root, and stops once the first deploy is healthy. Sections 1 to 3 stay the reference: they describe what the script does and how to do it by hand.

Before running it:

- an OVHcloud VPS with Debian 13 and your SSH key (section 1.1);
- the Object Storage bucket(s) and an S3 user with read/write access (section 2);
- a Cloudflare Origin CA certificate and key covering both hosts (section 1.9), copied to the server or ready to paste;
- an image published on the deploy branch (`rewrite-go`, CI job `build`) in the public GHCR package. Without it the first deploy fails; push, wait for CI, re-run.

Run it as root after reading it. Pin everything to one reviewed commit: the script downloads the agent, the deploy scripts, the systemd units and the env examples from `AWT_REPO_RAW`, which defaults to the moving branch; set it to the same commit SHA so the files it installs are the ones you reviewed (the script warns when `AWT_REPO_RAW` is not a 40-hex commit SHA):

```sh
sha=<40-hex commit SHA you reviewed>
curl -fsSLO --proto '=https' https://raw.githubusercontent.com/fatpat/acroworldtour.com/$sha/deploy/init-vps.sh
less init-vps.sh          # review what you are about to run as root
sudo -i                   # the script needs root
AWT_REPO_RAW=https://raw.githubusercontent.com/fatpat/acroworldtour.com/<same sha> bash /home/<you>/init-vps.sh
```

The script does not set up SSH: deploys are pulled by the host (section 5), so SSH access stays exactly as you or the provider set it up (see "Still by hand" below).

In a terminal it prompts for each missing value (secrets are not echoed; the certificate and key can be given as a file path or pasted). Without a terminal (`</dev/null`, CI, a pipe) it never prompts: it lists every missing or invalid value at once and stops before changing anything. One check needs the host half set up: whether a bucket holds a Litestream replica (MFA key rule below) runs in a container of the app image, so it happens after the packages, firewall and Docker steps. If it stops there (a replica and no MFA key, or the bucket cannot be listed), those steps stay applied; set the value and re-run, it resumes. To run it non-interactively, put the values in a root-only file instead of the shell history, load it, run, then delete the file:

```sh
install -m 600 /dev/null /root/awt-init.env && editor /root/awt-init.env   # NAME=value lines
set -a; . /root/awt-init.env; set +a
bash init-vps.sh </dev/null
rm /root/awt-init.env
```

`bash init-vps.sh --list-inputs` prints every input name. Inputs (env file keys are in `/opt/awt`, section 2):

| Variable | Default | Stored in | Notes |
|---|---|---|---|
| `AWT_PROD_HOST` | required | `.env` | Host name, for example `www.example.org`. `.env.prod` gets `AWT_BASE_URL=https://<host>`. |
| `AWT_TRAINING_HOST` | required | `.env` | Must differ from the prod host. |
| `AWT_IMAGE` | `ghcr.io/fatpat/awt` | `.env` | Image name without tag. |
| `AWT_CHANNEL` | `rewrite-go` | `.env` | Moving tag the agent follows. |
| `AWT_REPO_RAW` | `https://raw.githubusercontent.com/fatpat/acroworldtour.com/$AWT_CHANNEL` | not stored | Where `deploy/agent.sh`, `deploy.sh`, `weekly-restore-test.sh`, the systemd units and the env examples are downloaded from (https only, redirects too). Pin it to a commit: `https://raw.githubusercontent.com/fatpat/acroworldtour.com/<sha>`; a branch prints a warning. |
| `AWT_TLS_CERT_FILE` | required (or pasted) | `certs/origin.pem` | Origin CA certificate (PEM). Checked: parses, not expired, covers both hosts. |
| `AWT_TLS_KEY_FILE` | required (or pasted) | `certs/origin.key` (600) | Its private key; must match the certificate. |
| `PROD_S3_ENDPOINT`, `TRAINING_S3_ENDPOINT` | required | `AWT_S3_ENDPOINT` in `.env.prod`, `.env.training` | `https://` only, for example `https://s3.gra.io.cloud.ovh.net`. |
| `PROD_S3_REGION`, `TRAINING_S3_REGION` | required | `LITESTREAM_S3_REGION` | For example `gra`. |
| `PROD_S3_BUCKET`, `TRAINING_S3_BUCKET` | required | `AWT_S3_BUCKET` | One bucket shared by both environments, or one each. Litestream replicates to `<prefix>litestream/` in it; uploads go under `<prefix>files/` (section 6). |
| `PROD_BUCKET_PREFIX`, `TRAINING_BUCKET_PREFIX` | `prod/`, `training/` | `AWT_BUCKET_PREFIX` | Prefix of everything the environment writes to its bucket. The defaults work with a shared bucket and with dedicated ones; with a dedicated bucket it may be empty (set the variable to an empty value). Segments of letters, digits, `.`, `_`, `-` separated by single `/`, no `.` or `..` segment; the trailing `/` is optional (stored with one). Never prompted. |
| `PROD_S3_ACCESS_KEY`, `TRAINING_S3_ACCESS_KEY` | required | `AWT_S3_ACCESS_KEY` | Secret: hidden prompt, masked in the output. |
| `PROD_S3_SECRET_KEY`, `TRAINING_S3_SECRET_KEY` | required | `AWT_S3_SECRET_KEY` | Secret. |
| `PROD_MFA_KEY`, `TRAINING_MFA_KEY` | generated, unless the bucket holds a replica | `AWT_MFA_KEY` | Base64 of 32 bytes, different per environment. See the MFA rule below. |
| `PROD_CLOUDFLARE_ZONE_ID`, `TRAINING_CLOUDFLARE_ZONE_ID` | optional | `AWT_CLOUDFLARE_ZONE_ID` | 32 hexadecimal characters; given with the token or not at all. |
| `PROD_CLOUDFLARE_TOKEN`, `TRAINING_CLOUDFLARE_TOKEN` | optional | `AWT_CLOUDFLARE_TOKEN` | Cache purge token. Prompted only before the env file exists. |
| `AWT_INIT_FORCE` | `0` | not stored | `1` replaces stored values (env files, certificate) with the ones given. Values not given are kept: the installed certificate is replaced only when `AWT_TLS_CERT_FILE` and `AWT_TLS_KEY_FILE` are given. The deployed image tags are never touched. |
| `AWT_INIT_ALLOW_OS` | `0` | not stored | `1` runs on another system than Debian 13 (tests only). |

Values are written as `KEY=value` lines: secrets must not contain spaces, quotes, `$`, `#` or a backslash.

**What it does**, one idempotent step at a time, each reporting `==> <step>: changed` or `already done`:

1. `preflight`: root, Debian 13. `inputs`: environment, values stored on the host, prompts, validation (nothing is changed before every value is valid).
2. `packages`: `ca-certificates`, `curl`, `gnupg`, `openssl`, `nftables`, `unattended-upgrades` (automatic security updates, section 1.6).
3. `firewall`: `/etc/nftables.d/awt.nft` and `/etc/nftables.conf` (see section 1.8), loaded and enabled at boot. SSH stays open on every port sshd is configured on (`sshd -T`; 22 when sshd is missing or `sshd -T` fails), plus the port of the SSH session running the script when it differs; no input changes this. The first load is done twice, first with policy accept so that conntrack tracks the open connections, then with the drop policy.
4. `docker`: Docker Engine and the compose plugin from Docker's apt repository (key fingerprint checked), Compose 2.22 or later, log rotation in `/etc/docker/daemon.json` (only if the file does not exist; section 7).
5. `layout`: user `deploy` (group `docker`, no login shell, no `authorized_keys`), `/opt/awt` and its directories with the owners and modes of section 1.7, the three scripts of `deploy/` from the repository (root, 755).
6. `tls`: the certificate and key in `/opt/awt/certs`.
7. `env_files`: `.env`, `.env.prod`, `.env.training` (owner `deploy`, mode 600) from the examples of the repository, written while holding the agent's lock (`/run/lock/awt-agent.lock`) so a running agent never writes `.env` at the same time. A new file gets every example key with the given values; an existing file only gets the keys it lacks. Temporary files holding secrets are created mode 600 (the script runs under `umask 077`).
8. `agent`: `awt-agent.service` and `awt-agent.timer` from the repository, timer enabled (section 3.1).
9. `first_deploy`: one agent run (`systemctl start awt-agent.service`), then `/healthz` of both apps, directly and through Caddy, as `deploy.sh` does.
10. `summary`: hosts, deployed revision, masked secrets (`****` and the last 4 characters), and for each database whether it was restored from the S3 replica, already held data, or is empty, with the commands of section 4 (`awt import`, `awt user add`) for each empty one.

**MFA key rule.** A stored key is kept. A given key is used. Otherwise the script asks Litestream, in a one-shot container of the app image with the environment's S3 settings (`litestream snapshots`), whether the bucket holds a replica under `<prefix>litestream/` (the path comes from `awt bucket-path litestream` in that container, as in the entrypoint). No replica: it generates a key (`openssl rand -base64 32`). A replica: the database restored from it may hold TOTP secrets encrypted with the existing key, and a new key would make every enrolled authenticator app unusable (section 7b), so it never generates one: it prompts for the existing key (from your secrets backup), or, without a terminal, stops and names the variable to set. If the bucket cannot be listed (wrong endpoint or credentials, image not published) it stops too.

**Database.** The first deploy starts each app on an empty volume: the entrypoint restores the Litestream replica when the bucket has one (section 6), otherwise the database starts empty and you run section 4 (legacy import) and section 3.3 (first admin); the summary prints the exact commands.

**Re-running** on a configured host changes nothing and asks for nothing that is already stored: every step reports `already done`. A run that stopped (invalid value, agent failure) can be re-run as is: finished steps are skipped and it resumes where it failed. When the first deploy fails, the script prints the end of `journalctl -u awt-agent` and the next steps; everything else stays installed and the timer keeps retrying every 60 s (a revision recorded in `state/failed-revisions` is skipped until you delete its line, section 5).

**Still by hand:**

- SSH access and its hardening are yours: the account, keys, `sudo`, sshd settings, port and `fail2ban` stay as you or the provider set them up (sections 1.2 and 1.3); the script neither installs nor configures any of them.
- The firewall keeps sshd's configured port(s) open (`sshd -T`, read at each run; 22 if it cannot be read) plus the port of the session running the script. So change the sshd port before running the script, or re-run it after changing it, keeping your current session open until the re-run is done; otherwise the new port stays closed.
- The provider's network firewall, if enabled (OVHcloud Network Firewall in the control panel), must allow your SSH port.
- Cloudflare (section 1.10): proxied DNS records for both hosts (`A` only: no `AAAA` record pointing to the origin), SSL mode Full (strict), a cache bypass rule for `/live/*`; the Origin CA certificate itself (section 1.9).
- GitHub: the repository, the deploy branch and the public GHCR package (section 2).
- An empty database: the legacy import (section 4) and the first admin (section 3.3).
- Back up `/opt/awt/.env.prod` and `/opt/awt/.env.training` (MFA keys, S3 secrets) with your other secrets.
- The weekly restore test cron (section 6) and uptime monitoring (section 7).

**Local test:** `make init-vps-test` runs the script as root in `debian:13` containers: packages, users, firewall files, layout, certificate, env files and units are produced for real; `systemctl`, `journalctl`, `nft` (loading rules needs `NET_ADMIN`) and `docker` (no daemon; the agent run is simulated) are stubbed. `nft` also parses the generated ruleset for real (without `NET_ADMIN` it stops at its kernel cache, after the syntax check; the check is complete where the Docker daemon allows `--cap-add NET_ADMIN`). sshd is absent, or a stub answering `sshd -T`. Cases: happy path (no sshd: SSH on 22, nothing SSH-related installed or written, two-phase firewall load, umask), re-run with no input (nothing changes), no SSH input (`AWT_ADMIN_*`, `AWT_SSH_PORT`) listed, required or prompted, default bucket prefixes, a prefix normalised (`prod` to `prod/`) or emptied with `AWT_INIT_FORCE=1` and kept on a re-run, missing values, invalid `http://` endpoint, MFA key and bucket prefixes, one TLS file only (interactive), wrong system, not root, the SSH rule following `sshd -T` (port 22, port 2200, several ports, `sshd -T` failing) and the session's port (kept, firewall already up, certificate kept with `AWT_INIT_FORCE=1`, then dropped from a session on sshd's port), mismatched certificate, S3 listing error with exit 0, replica without MFA key (non-interactive and prompted), failed first deploy then resume, and every input name documented here. It needs Docker and network and takes a few minutes; it is not part of `make check`.

## 1. Provision the VPS

Section 0 automates sections 1 to 3; they remain the reference for what it does and for a manual setup.


1. Order an OVHcloud VPS with Debian 13 (required by `deploy/init-vps.sh`; the manual steps also work on Debian 12 or Ubuntu 24.04 LTS). Add your SSH public key at order time.
2. Create a non-root deploy user `deploy` in the `docker` group (see the note on root equivalence below). SSH access is yours to set up and `deploy/init-vps.sh` leaves it alone: keep what the provider set up, or add an admin user for yourself and harden SSH by hand, for example in `/etc/ssh/sshd_config.d/10-hardening.conf`, then `systemctl reload ssh` (if you change the port, do it before running the script or re-run it afterwards, section 1.8):
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
8. Firewall: allow SSH (sshd's port), allow 80 and 443 from Cloudflare ranges only, deny everything else inbound. Docker bypasses ufw and the input chain for published ports: it DNATs them in prerouting, so that traffic goes through the forward hook. `deploy/init-vps.sh` uses nftables directly (no ufw): `/etc/nftables.d/awt.nft` holds one table `inet awt` with sets `cloudflare4`/`cloudflare6`, an `input` chain (policy drop: established, loopback, ICMP, DHCP replies, SSH from anywhere on the ports below, tcp/80, tcp/443 and udp/443 from Cloudflare) and a `forward` chain at priority `filter - 10` that drops new connections whose original destination port is 80 or 443 (`ct status dnat ct original proto-dst`) unless the source is a Cloudflare address. A drop in any base chain is final in nftables, so this holds whatever Docker's own chains accept, and Docker never touches the `awt` table. Connections taken by Docker's userland proxy (for example IPv6 to the host) reach the `input` chain and get the same rule; but the proxy then connects to Caddy from the Docker bridge gateway, so Caddy sees that address as the peer and its Cloudflare-only check rejects the request. The origin is therefore reachable over IPv4 only: give Cloudflare no `AAAA` record for the hosts (it serves IPv6 to visitors anyway).

SSH ports: the script does not manage SSH (section 0), so the firewall must follow whatever sshd does and never lock the operator out; the rule depends on no input. At each run it allows TCP to every port of the `port` lines of `sshd -T` (22 when sshd is missing or `sshd -T` fails), plus the server port of the SSH session running the script (`SSH_CONNECTION`, or the parent processes' environment under `sudo -i`) when it is not one of them, so that session keeps working whatever happens (for example a port added by hand to `ssh.socket`, which `sshd -T` does not show). The list is recomputed at every run: after moving sshd to another port, re-run the script (the new port is opened; the old one stays open only while the run comes from a session on it), or change the port before the first run. On the first load the script loads the same table with policy accept, waits 2 s, then loads the drop policy: on a fresh host no rule uses conntrack yet, so the open connections have no conntrack entry and their next packet would count as `ct state new` and be dropped unless a port rule matches it; the SSH rules cover the session running the script when its port is known, the two-phase load also keeps a session whose port could not be found (no `SSH_CONNECTION` in its process tree) and replies to outbound connections. `/etc/nftables.conf` only includes `/etc/nftables.d/*.nft`, without Debian's `flush ruleset` (which would delete Docker's tables on every reload; the original is kept as `/etc/nftables.conf.dist`), and a drop-in makes `systemctl stop nftables` delete only the `awt` table. Reload after a change with `nft -f /etc/nftables.conf`. The OVHcloud network firewall in front of the VPS is an alternative. Cloudflare ranges: https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6 (the script fetches them and accepts the lists only if every entry is an IPv4 network from /8 to /32 or an IPv6 network from /16 to /128; otherwise it keeps the sets already in `awt.nft`, or, on a first run, uses a list pinned in it). The Caddyfile also aborts any request whose peer is not a Cloudflare address (defence in depth). Refresh the CIDR list in the firewall (re-run the script) and in the `Caddyfile` when Cloudflare changes it (check quarterly).
9. **TLS strategy: Cloudflare Origin CA certificate, SSL mode Full (strict).** In Cloudflare, create an Origin CA certificate covering the prod and training hosts (or `*.example.org`), validity up to 15 years. Save it as `/opt/awt/certs/origin.pem` and the private key as `/opt/awt/certs/origin.key` (`chmod 700 certs`, key `600`; if Caddy cannot read the key because Docker runs with user namespaces or rootless, use `640` with a group Caddy shares, or `644` inside the 700 directory). Paths can be changed with `AWT_TLS_CERT` and `AWT_TLS_KEY` in the Caddy environment. Caddy loads these files instead of using ACME, so no certificate is requested from Let's Encrypt, and nothing but Cloudflare can complete a trusted TLS handshake with the origin. The certificate is not trusted by browsers; the origin must never be exposed without Cloudflare. Replace it before it expires: write the new files and run `docker compose restart caddy`.
10. Cloudflare: proxied DNS records for the prod and training hosts, `A` records only (no `AAAA` to the origin, see 8). Do not cache `/live/*` (add a cache rule: bypass). Cloudflare closes idle connections after about 100 s; the live stream sends a heartbeat about every 30 s (spec 6.4) and Caddy does not buffer `/live/*`.

## 2. Secrets and configuration

Nothing secret is committed. On the VPS, in `/opt/awt` (mode 600, owner = deploy user):

| File | From | Holds |
|---|---|---|
| `.env` | `deploy/env.example` | hosts, image name, moving tag the agent follows (`AWT_CHANNEL`, default `rewrite-go`), deployed image tags (rewritten by `deploy.sh`; `none` = nothing deployed yet, there is no `latest` default) |
| `.env.prod` | `deploy/env.prod.example` | `AWT_*` for prod: base URL (required), S3 keys, Cloudflare token, trusted proxies |
| `.env.training` | `deploy/env.training.example` | same for training, with its own base URL and bucket or bucket prefix |

There is no session secret: sessions are random tokens stored in the database. `AWT_ENV` is set per service in `compose.yaml` (`prod`, `training`); the app refuses to start outside dev without `AWT_BASE_URL`. Set `AWT_IMAGE` in `.env` to `ghcr.io/<lowercase-owner>/awt`. Create the Object Storage bucket(s) and an S3 user with read/write access to them. `LITESTREAM_S3_REGION` is the OVH region (for example `gra`). Leave `AWT_S3_BUCKET` empty to run without replication (not for prod).

**Bucket prefix.** `AWT_BUCKET_PREFIX` (`prod/` and `training/` in the examples) is put in front of every object key the environment writes: the replica in `<prefix>litestream/`, uploads in `<prefix>files/`. So both environments can share one bucket (different prefixes), or each have its own (any prefix, or empty). With or without a trailing `/`; letters, digits, `.`, `_`, `-` and single `/` only, no `.` or `..` segment. The app refuses to start on an invalid value, and so does the container: the entrypoint gets the replica path from `awt bucket-path litestream`, which applies the same rule (decision 0017). `AWT_ENV` is not part of any bucket path.

**Manager MFA key.** TOTP secrets are stored encrypted with `AWT_MFA_KEY` (base64, 32 bytes). Generate it once per environment with `openssl rand -base64 32` and put it in `.env.prod` / `.env.training` (different values). The app refuses to start outside dev if any TOTP secret exists and the key is missing; if the key is lost or changed, enrolled TOTP secrets become unusable and each affected manager must be reset by an admin and re-enrol. Back it up with the other secrets. In dev a fixed key is used and a warning is logged.

**WebAuthn RP ID.** `AWT_WEBAUTHN_RP_ID` defaults to the host of `AWT_BASE_URL` (`localhost` in dev). Set it only to share passkeys across subdomains (a parent domain of the site host). Changing it later invalidates every registered passkey, so decide before managers enrol. The accepted WebAuthn origin is the origin of `AWT_BASE_URL` (in dev without it, `http://localhost:<port>` and `http://127.0.0.1:<port>` from `AWT_ADDR`), so security keys only work when the manager is opened at that address.

Deploys need no GitHub secret and no registry credential on the VPS: the repository and the GHCR package are public, so the agent, `deploy.sh` and the restore test pull anonymously. If the package is ever made private, run `docker login ghcr.io` once as `deploy` with a read-only personal access token.

## 3. First deploy

1. Provision and configure as above, then install the agent (as root; `deploy/init-vps.sh` does this and runs the agent once, section 0):
   ```sh
   install -m 644 awt-agent.service awt-agent.timer /etc/systemd/system/   # from deploy/ in the repository
   systemctl daemon-reload
   systemctl enable --now awt-agent.timer
   ```
   Push to the deploy branch (`rewrite-go`, set in the `if:` of the `build` job in `.github/workflows/ci.yml`; the moving tag has the same name). After `check` passes, `build` publishes the image; within a minute the agent installs its `compose.yaml` and `Caddyfile`, deploys training (health checked), then prod. Follow it with `journalctl -u awt-agent -f`. The first run starts empty databases (migrations run at `awt serve` startup).
2. Check: `docker compose ps` shows `healthy` for `awt-prod` and `awt-training`; `docker compose logs awt-prod` shows `listening`; the public URL answers through Cloudflare; the bucket gets a `<prefix>litestream/` prefix within a minute (`prod/litestream/` with the example prefix).
3. Create the first admin: `docker compose exec awt-prod awt user add --email you@example.org --name "You" --role admin` (the password is read from stdin or prompted).

## 4. First import (legacy data)

Production data lives in the MongoDB export (`mongo.backup.*.tar.gz`, never committed). Import on the VPS into an empty database:

```sh
scp mongo.backup.YYYYMMDD-HHMM.tar.gz <you>@<vps-host>:/tmp/   # add -P <port> if sshd is not on 22
ssh <you>@<vps-host>     # your own SSH access; deploy has no SSH access
cd /opt/awt
docker compose cp /tmp/mongo.backup.*.tar.gz awt-training:/tmp/export.tar.gz
docker compose exec awt-training awt import --from /tmp/export.tar.gz
```

Then import the legacy institutional site: news, events, the sporting code PDFs and the redirects of the old `.html` URLs (spec 07 §10). `awt import` does not bring them. Build the archive on your machine from the legacy repository (`legacy/legacy` here, or the `main` branch of the GitHub repository), then:

```sh
tar -czf www.tar.gz -C legacy/legacy www                      # on your machine
scp www.tar.gz <you>@<vps-host>:/tmp/
ssh <you>@<vps-host>
cd /opt/awt
docker compose cp /tmp/www.tar.gz awt-training:/tmp/www.tar.gz
docker compose exec awt-training sh -c 'tar -xzf /tmp/www.tar.gz -C /tmp && awt import-www --from /tmp/www'
```

Undated news take the date of the commit that added them to the legacy repository, recorded in the image (`internal/importer/legacy_news_dates.tsv`), so the archive needs no git history. The report lists any news left as a draft: set its date in the manager to publish it. Re-running `import-www` overwrites manager edits to the same news and events, so run it once.

Run both imports on training first and check the result in the browser. Then repeat on prod (`awt-prod`) before opening the site. The imports write through the normal database connection, so Litestream replicates them. Delete the archives from the VPS and the container (`rm -r /tmp/export.tar.gz /tmp/www.tar.gz /tmp/www`) afterwards.

## 5. Routine deploys and rollback

Every push to the deploy branch that passes `check` publishes a new image; the agent deploys it automatically, training then prod, with the same revision. One pass of `deploy/agent.sh` (systemd timer, every 60 s, one run at a time through `flock` on `/run/lock/awt-agent.lock`):

1. Pulls `$AWT_IMAGE:$AWT_CHANNEL`. A failed pull (registry down, network) logs one error and changes nothing; the next tick retries.
2. Reads the commit from the `org.opencontainers.image.revision` label. An image without a 40-character commit id (built outside CI) is refused and nothing is deployed.
3. If training and prod already run that revision, stops silently (no restart).
4. If the revision is listed in `/opt/awt/state/failed-revisions`, logs `skip <rev>: failed before` and stops.
5. Copies `compose.yaml` and `Caddyfile` out of the image and validates them (`docker compose config` with `.env`; `caddy validate` with the Caddy image of the new `compose.yaml`, the hosts from `.env` and `/opt/awt/certs`). Invalid: the revision is recorded as failed and the installed files stay as they are.
6. Saves the current files and deployed tags (`/opt/awt/state/prev/`), installs the two files (write then rename), tags the pulled image as `:<rev>` locally (so `deploy.sh` does not pull again), runs `deploy.sh training <rev>`, then, only if training succeeded, `deploy.sh prod <rev>`. A failure of either (app not healthy, or Caddy validate/reload/health check failing after a healthy app) records the revision as failed; a failed pull inside `deploy.sh` (exit 3) is not recorded and retried next tick. In every failure case after the install (including a failed command, `INT`/`TERM` or the systemd start timeout) the agent puts the previous `compose.yaml` and `Caddyfile` back and redeploys the previous training revision (and the previous prod revision if prod was attempted), so both apps and Caddy end on the previous revision with its own config. A restore that itself fails logs `RESTORE FAILED`: check by hand. If a run is killed outright (`SIGKILL`, power loss) between install and the end, the next run notices it (`previous run did not finish`; `state/installed-ok` is missing) and keeps the saved `state/prev/` instead of saving the half-installed files over it. A run killed after both deploys succeeded but before writing the marker is fine: the next no-op run (both tags already on the revision) writes it back.
7. After a successful deploy, removes local images of `$AWT_IMAGE` older than `AWT_PRUNE_DAYS` days (environment of the service, an integer from 0 to 9999 without leading zeros, otherwise 30 with a warning; default 30) except the new revision, the previous training and prod revisions and the moving tag (`$AWT_CHANNEL`), then this project's dangling images as old. Pruning never fails a run.

`deploy.sh <env> <tag>` waits up to 120 s for the container health check, then requests `/healthz` inside the stack (directly on the app container, then through Caddy on its loopback-only listener `127.0.0.1:8081` in the `caddy` container, which is never published). Caddy's config is validated and reloaded on every deploy; a validation or reload failure fails the deploy (no silent success). If the new app container is not healthy it restarts the previous tag and exits non-zero. On the very first deploy there is no previous tag and nothing to roll back to.

Operating the agent (as an admin, with `sudo`):

- **Watch:** `journalctl -u awt-agent -f` (outcomes are only in the VPS logs; CI only reports the image as published). `systemctl list-timers awt-agent.timer` shows the next run.
- **Retry a failed revision:** delete its line from `/opt/awt/state/failed-revisions`; the next tick deploys it again. Pushing a new commit also deploys normally (only the failed revision is skipped).
- **Pause deploys:** `systemctl stop awt-agent.timer` (`systemctl start awt-agent.timer` to resume). A run in progress finishes.
- **Manual rollback** (for example a bad release that is healthy but wrong): first `systemctl stop awt-agent.timer`, otherwise the agent redeploys the moving tag within a minute; then `sudo -u deploy /opt/awt/deploy/deploy.sh prod <previous-sha>` (and `training` if needed). Tags are the commit SHAs in the GHCR package list. Restart the timer once a fixed commit is published (or record the bad revision in `failed-revisions` first). Migrations only move forward and must stay compatible with the previous release for one version; if a migration itself is the problem, restore the database (next section).
- **Updating the agent or `deploy.sh`:** copy the new script to `/opt/awt/deploy/` by hand (root-owned, `chmod 755`); they are not shipped by the image.

**Local test:** `make agent-test` runs the agent against a local registry and a temporary `/opt/awt` through the real `deploy.sh` and `compose.yaml`: happy path, no-op, lock held, bad revision (rollback, previous config restored, recorded, skipped next run), invalid baked Caddyfile or compose.yaml, image with an `unknown` or no revision label, registry down then back, pull failing inside `deploy.sh` (not recorded, retried), `:<rev>` missing from the registry but present locally, prod failing after a healthy training (both restored). It needs Docker and takes a few minutes; it is not part of `make check`.

## 6. Backups and restore

**Bucket layout.** Each environment writes under its `AWT_BUCKET_PREFIX` (section 2; `<prefix>` below, possibly empty), in its own bucket or in one bucket shared with the other environment:

- `<prefix>litestream/`: the database replica (below).
- `<prefix>files/<sha256>`: uploaded photos, logos and PDFs. Each object is named by the SHA-256 of its content and carries its content type. The original filename lives in the database's `files` table. Litestream does not back these up, so never delete them by hand; the database refers to them.

For example, one bucket for both: `prod/litestream/`, `prod/files/`, `training/litestream/`, `training/files/`. A dedicated bucket with an empty prefix: `litestream/`, `files/`.

**Moving from the old layout.** Images built before this layout stored the files at the bucket root and the replica in `litestream/<env>/`.

Files: copy the bucket's root-level files to `<prefix>files/` once per environment, before the new image deploys. The copy leaves the old objects in place, so the running app keeps working. `<old-bucket>` is the bucket the environment used so far; `<bucket>` is its bucket from now on (the same one, or a shared one):

```sh
AWS='docker run --rm -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION=gra
  amazon/aws-cli@sha256:e3e329e1d2894b7b4bbb0aacacd0a155262159b2e7a3b4275eb1f24046d3e06c
  --endpoint-url https://s3.gra.io.cloud.ovh.net'
HEX=$(printf '[0-9a-f]%.0s' $(seq 64))   # matches only the 64-hex names at the root
PREFIX=prod/                             # this environment's AWT_BUCKET_PREFIX, with its trailing /, or empty
read -r AWS_ACCESS_KEY_ID; read -rs AWS_SECRET_ACCESS_KEY; export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
$AWS s3 cp s3://<old-bucket>/ "s3://<bucket>/${PREFIX}files/" --recursive --exclude '*' --include "$HEX"
```

Once the new image is deployed and images load on the site, remove the old copies:

```sh
$AWS s3 rm s3://<old-bucket>/ --recursive --exclude '*' --include "$HEX"
```

Litestream: nothing to move on a running server. The volume already holds the database, so when the new image starts, Litestream finds no replica at `<prefix>litestream/` and starts a fresh one there from the local database (check that it appears, section 3). Once it exists, delete the old replica:

```sh
$AWS s3 rm s3://<old-bucket>/litestream/<env>/ --recursive   # <env> = prod or training
```

With an empty prefix in the same bucket, the new replica is `litestream/generations/` and the old one `litestream/<env>/`: the command above only removes the old one. The one caveat: between the deploy and the first snapshot at the new path, a restore on an empty volume (section 6 below, or the weekly restore test) finds no replica, so do not wipe a volume in that window.

These commands were checked against a local MinIO (same `aws-cli` image): only the 64-hex root objects are copied and removed, other keys and the other environment's objects are left alone, with the same bucket or another one as the target.

Litestream replicates the database continuously (about 1 s lag), snapshots every 6 h, keeps 30 days (`litestream.yml`).

**Automatic restore on an empty volume.** The entrypoint runs `litestream restore -if-db-not-exists -if-replica-exists` before starting. To rebuild a lost or corrupted environment:

```sh
cd /opt/awt
docker compose stop awt-prod
docker compose rm -f awt-prod
docker volume rm awt_prod_data
docker compose up -d awt-prod          # restores from the replica, then serves
```

**Point in time restore** to a scratch file (does not touch prod). Litestream commands run outside the entrypoint (`--entrypoint sh`, `docker compose exec`) do not see the `LITESTREAM_REPLICA_PATH` it exports: set it from `awt bucket-path litestream` first, as below; without it, `litestream.yml` points at the bucket root.

```sh
docker compose run --rm --no-deps --entrypoint sh awt-prod -c \
  'p=$(awt bucket-path litestream) && export LITESTREAM_REPLICA_PATH=$p && litestream restore -config /etc/litestream.yml -timestamp 2026-10-18T12:00:00Z -o /tmp/pit.db /data/awt.db && ls -l /tmp/pit.db'
```

To make a point in time restore live, restore to `/data/awt.db` on an emptied volume (same command with `-o /data/awt.db` inside a container mounting the volume), then start the service.

**Local drill:** `make backup-restore-test` runs the whole cycle against a local MinIO in a few seconds (writes data, deletes the volume, starts again, checks the data is back). It runs with `AWT_BUCKET_PREFIX=restore-test/` and checks the replica lands under `restore-test/litestream/` and not at the bucket root.

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
