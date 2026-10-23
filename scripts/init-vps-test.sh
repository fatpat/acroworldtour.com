#!/usr/bin/env bash
# VPS initialization test: runs deploy/init-vps.sh as root inside debian:13
# containers (pinned by digest). Packages (apt, Docker's apt repository),
# users, firewall files, layout, certificates, env files and systemd units
# are produced for real. What a container cannot do is stubbed through PATH
# (see the stubs below): systemctl (no systemd; the deploy agent run is
# simulated), journalctl, nft (loading a ruleset needs NET_ADMIN; the
# generated ruleset is still parsed by the real nft, see nft_parses), and
# docker (no daemon: the real CLI is installed by apt, the stub answers in its
# place). sshd is absent (the script installs no SSH server); where a case
# needs one, a stub answering `sshd -T` is installed in that container.
# Repository files are served from this checkout (AWT_REPO_RAW=file:///src).
#
# Cases: every input documented in docs/ops.md, no SSH input (AWT_ADMIN_*,
# AWT_SSH_PORT) listed, required or prompted, happy path (no sshd: SSH rule
# on 22), default bucket prefixes, re-run (nothing changes), missing values
# (non-interactive), invalid inputs (http S3 endpoint, MFA key, bucket
# prefixes), bucket prefix normalised or emptied with AWT_INIT_FORCE=1 and
# kept on a re-run, one TLS file only (interactive), wrong
# OS, not root, firewall SSH rule following `sshd -T` (port 22, 2200, several
# ports, sshd -T failing) plus the port of the SSH session running the
# script (kept, then dropped from a session on sshd's port), mismatched TLS
# pair, S3 listing error reported with exit 0, S3 replica without MFA key
# (refused non-interactive, prompted interactive), failed first deploy then
# resume.
# Run via `make init-vps-test` (needs Docker and network; takes a few minutes).
set -euo pipefail
cd "$(dirname "$0")/.."
repo=$(pwd)

DEBIAN_IMAGE=debian:13@sha256:4c25275446ca371766b8b093c696782eabce06263fccacb8d5d813b3dc44b308
PREFIX=awt-init-vps-test-$$
APT_CACHE=$PREFIX-apt
REV=abcdefabcdefabcdefabcdefabcdefabcdefabcd
work=$(mktemp -d)
chmod 755 "$work"   # read by the container's root (may be a remapped user)
failures=0
ctrs=()
ctr=""

cleanup() {
  for c in "${ctrs[@]}"; do docker rm -f "$c" >/dev/null 2>&1 || true; done
  docker volume rm -f "$APT_CACHE" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

ok() { echo "  ok: $*"; }
bad() { echo "  FAIL: $*" >&2; failures=$((failures + 1)); }
check() { # check <description> <command...>
  local d=$1; shift
  if "$@"; then ok "$d"; else bad "$d"; fi
}
has() { printf '%s' "$out" | grep -Fq -- "$1"; }
hasnt() { ! has "$1"; }

echo "== fixtures"
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 30 \
  -subj /CN=example.org -addext 'subjectAltName=DNS:example.org,DNS:*.example.org' \
  -keyout "$work/origin.key" -out "$work/origin.pem" 2>/dev/null
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$work/other.key" 2>/dev/null
chmod 644 "$work"/*
MFA_GIVEN=$(openssl rand -base64 32)

mkdir "$work/stubs"
# systemctl: no systemd in the container. Records calls, keeps enabled/active
# state per unit, and simulates one deploy agent run on
# `start awt-agent.service` (fails when /var/lib/awt-test/agent-fail exists).
cat > "$work/stubs/systemctl" <<'EOF'
#!/bin/sh
t=/var/lib/awt-test; st=$t/systemctl; mkdir -p "$st"
echo "systemctl $*" >> "$t/calls.log"
now=0; cmd=""; units=""
for a in "$@"; do
  case "$a" in
    --now) now=1 ;;
    -*) ;;
    *) if [ -z "$cmd" ]; then cmd=$a; else units="$units $a"; fi ;;
  esac
done
agent_run() {
  echo run >> "$t/agent-runs"
  [ -x /opt/awt/deploy/agent.sh ] && [ -f /opt/awt/.env.prod ] && [ -f /opt/awt/.env.training ] \
    || { echo "agent: not installed" >> "$t/journal"; return 1; }
  if [ -e "$t/agent-fail" ]; then
    echo "agent: pull ghcr.io/fatpat/awt:rewrite-go failed: simulated failure" >> "$t/journal"
    return 1
  fi
  new=$(sed -e "s/^AWT_TRAINING_TAG=.*/AWT_TRAINING_TAG=$REV/" -e "s/^AWT_PROD_TAG=.*/AWT_PROD_TAG=$REV/" /opt/awt/.env)
  printf '%s\n' "$new" > /opt/awt/.env   # in place: owner and mode kept, as deploy.sh as deploy
  echo "agent: deployed $REV to training and prod" >> "$t/journal"
}
rc=0
case "$cmd" in
  enable) for u in $units; do : > "$st/enabled.$u"; [ "$now" = 0 ] || : > "$st/active.$u"; done ;;
  disable) for u in $units; do rm -f "$st/enabled.$u"; [ "$now" = 0 ] || rm -f "$st/active.$u"; done ;;
  start|restart|reload|reload-or-restart|try-reload-or-restart)
    for u in $units; do
      if [ "$u" = awt-agent.service ]; then agent_run || rc=1; else : > "$st/active.$u"; fi
    done ;;
  stop) for u in $units; do rm -f "$st/active.$u"; done ;;
  is-enabled) for u in $units; do if [ -e "$st/enabled.$u" ]; then echo enabled; else echo disabled; rc=1; fi; done ;;
  is-active) for u in $units; do if [ -e "$st/active.$u" ]; then echo active; else echo inactive; rc=3; fi; done ;;
esac
exit $rc
EOF
sed -i "s/\$REV/$REV/g" "$work/stubs/systemctl"
cat > "$work/stubs/journalctl" <<'EOF'
#!/bin/sh
echo "journalctl $*" >> /var/lib/awt-test/calls.log
cat /var/lib/awt-test/journal 2>/dev/null || true
EOF
# nft: checking or loading a ruleset needs NET_ADMIN (refused here). Each
# loaded file is kept as /var/lib/awt-test/nft-load.<n>.
cat > "$work/stubs/nft" <<'EOF'
#!/bin/sh
t=/var/lib/awt-test
echo "nft $*" >> "$t/calls.log"
case "$*" in
  "-c -f "*) exit 0 ;;
  "-f "*)
    n=$(ls "$t"/nft-load.* 2>/dev/null | wc -l)
    cp "$2" "$t/nft-load.$((n + 1))"
    : > "$t/nft-loaded" ;;
  "list table inet awt") [ -e "$t/nft-loaded" ] ;;
  *) exit 0 ;;
esac
EOF
# docker: no daemon. Absent until apt installed the real CLI (so the script
# installs Docker for real); then answers for it. /var/lib/awt-test/replica-<env>
# = the env's bucket holds a Litestream replica; /var/lib/awt-test/s3-error =
# litestream logs an S3 error and still exits 0. Records the caller's umask.
cat > "$work/stubs/docker" <<'EOF'
#!/bin/sh
t=/var/lib/awt-test
[ -x /usr/bin/docker ] || { echo "docker: command not found (stub: not installed)" >&2; exit 127; }
echo "docker $*" >> "$t/docker.log"
umask >> "$t/umask.log"
if [ "$1" = compose ]; then
  shift
  while [ $# -gt 0 ]; do
    case "$1" in --project-directory|-f|--env-file|-p) shift 2 ;; *) break ;; esac
  done
  sub=$1; shift
  case "$sub" in
    version) case "$*" in *--short*) echo 2.40.3 ;; *) echo "Docker Compose version v2.40.3" ;; esac ;;
    ps) echo 0123456789ab ;;
    exec)
      [ "$1" = -T ] && shift
      svc=$1; shift
      case "$*" in
        *sqlite3*) env=${svc#awt-}; if [ -e "$t/replica-$env" ]; then echo 3; else echo 0; fi ;;
        *wget*) exit 0 ;;
      esac ;;
    logs)
      for a in "$@"; do svc=$a; done
      env=${svc#awt-}
      if [ -e "$t/replica-$env" ]; then
        echo 'time=2026-10-23T10:00:00Z level=INFO msg="restoring snapshot" db=/data/awt.db replica=s3'
      else
        echo 'time=2026-10-23T10:00:00Z level=INFO msg="no matching backups found"'
      fi ;;
  esac
  exit 0
fi
case "$1" in
  run)
    env=""
    # The environment, from its bucket prefix (the defaults prod/, training/).
    for a in "$@"; do case "$a" in AWT_BUCKET_PREFIX=*) env=${a#AWT_BUCKET_PREFIX=}; env=${env%/} ;; esac; done
    case "$*" in
      *snapshots*)
        echo "replica  generation        index  size  created"
        if [ -e "$t/s3-error" ]; then
          echo 'time=2026-10-23T10:00:00Z level=ERROR msg="cannot determine snapshots" err="operation error S3: ListObjectsV2, https response error StatusCode: 403"' >&2
          exit 0
        fi
        if [ -e "$t/replica-$env" ]; then echo "s3       f15d9eae2e6949aa  0      354   2026-10-01T10:16:44Z"; fi ;;
    esac ;;
esac
exit 0
EOF
chmod 755 "$work/stubs"/*
# sshd: not on PATH by default (the script installs no SSH server). Installed
# as /usr/local/sbin/sshd by the cases that need one: `sshd -T` prints
# /var/lib/awt-test/sshd-T, or fails when /var/lib/awt-test/sshd-fail exists.
cat > "$work/sshd-stub" <<'EOF'
#!/bin/sh
t=/var/lib/awt-test
echo "sshd $*" >> "$t/calls.log"
[ "$1" = -T ] || exit 0
if [ -e "$t/sshd-fail" ]; then echo "sshd: stub failure" >&2; exit 255; fi
cat "$t/sshd-T"
EOF
chmod 755 "$work/sshd-stub"

# start_ctr <name>: a fresh debian:13 "VPS" (sets $ctr).
start_ctr() {
  ctr=$PREFIX-$1
  docker run -d --name "$ctr" -v "$repo:/src:ro" -v "$work:/work:ro" \
    -v "$APT_CACHE:/var/cache/apt/archives" "$DEBIAN_IMAGE" sleep infinity >/dev/null
  ctrs+=("$ctr")
  # Container quirks, not VPS state: keep downloaded .debs (shared cache),
  # never start services from maintainer scripts.
  docker exec "$ctr" sh -c 'rm -f /etc/apt/apt.conf.d/docker-clean
    printf "#!/bin/sh\nexit 101\n" > /usr/sbin/policy-rc.d && chmod 755 /usr/sbin/policy-rc.d
    mkdir -p /var/lib/awt-test'
}
in_ctr() { docker exec -i "$ctr" "$@"; }

BASE_ENV=(-e PATH=/work/stubs:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
  -e AWT_INIT_ALLOW_OS=1 -e AWT_REPO_RAW=file:///src)
FULL=(-e AWT_PROD_HOST=prod.example.org -e AWT_TRAINING_HOST=training.example.org
  -e PROD_S3_ENDPOINT=https://s3.gra.io.cloud.ovh.net -e PROD_S3_REGION=gra -e PROD_S3_BUCKET=awt-prod
  -e PROD_S3_ACCESS_KEY=prodaccess0001 -e PROD_S3_SECRET_KEY=prod-secret-value-9876
  -e PROD_CLOUDFLARE_ZONE_ID=0123456789abcdef0123456789abcdef -e PROD_CLOUDFLARE_TOKEN=cf-token-value-5555
  -e TRAINING_S3_ENDPOINT=https://s3.gra.io.cloud.ovh.net -e TRAINING_S3_REGION=gra -e TRAINING_S3_BUCKET=awt-training
  -e TRAINING_S3_ACCESS_KEY=trainaccess0002 -e TRAINING_S3_SECRET_KEY=training-secret-value-4321
  -e AWT_TLS_CERT_FILE=/work/origin.pem -e AWT_TLS_KEY_FILE=/work/origin.key)

# run_init <docker exec options...>: non-interactive run (stdin is not a TTY).
run_init() {
  set +e
  out=$(docker exec -i "${BASE_ENV[@]}" "$@" "$ctr" bash /src/deploy/init-vps.sh </dev/null 2>&1)
  rc=$?
  set -e
  printf '%s\n' "$out" | sed 's/^/    | /'
  echo "    (exit $rc)"
}
# run_init_tty <stdin text> <docker exec options...>: interactive run in a pty.
run_init_tty() {
  local input=$1; shift
  set +e
  out=$(printf '%s' "$input" | docker exec -i "${BASE_ENV[@]}" "$@" "$ctr" \
    script -qec 'bash /src/deploy/init-vps.sh' /dev/null 2>&1)
  rc=$?
  set -e
  printf '%s\n' "$out" | sed 's/^/    | /'
  echo "    (exit $rc)"
}

stat_of() { in_ctr stat -c '%U:%G %a' "$1" 2>/dev/null || echo missing; }
file_has() { in_ctr grep -Eq -- "$2" "$1"; }
exists() { in_ctr test -e "$1"; }
# Buffered: `calls | grep -q` must not cut docker exec's output stream short
# (pipefail would then fail the check).
calls() { local c; c=$(in_ctr cat /var/lib/awt-test/calls.log 2>/dev/null || true); printf '%s\n' "$c"; }
agent_runs() { in_ctr sh -c 'wc -l < /var/lib/awt-test/agent-runs 2>/dev/null || echo 0'; }
env_value() { { in_ctr sed -n "s/^$2=//p" "$1" 2>/dev/null || true; } | tail -1; }
same_as_repo() { in_ctr cmp -s "/src/$1" "$2"; }
# Every key of the example is in the installed env file.
all_keys() {
  local k
  for k in $(sed -n 's/^\([A-Z_][A-Z0-9_]*\)=.*/\1/p' "deploy/$1"); do
    in_ctr grep -q "^$k=" "$2" || { echo "    missing $k in $2" >&2; return 1; }
  done
}
b64_32() { [ "$(printf '%s' "$1" | base64 -d 2>/dev/null | wc -c)" = 32 ]; }
steps_already_done() {
  local s
  for s in packages firewall docker layout tls env_files agent first_deploy; do
    has "==> $s: already done" || { echo "    step $s not 'already done'" >&2; return 1; }
  done
  ! printf '%s' "$out" | grep -q '^==> .*: changed$'
}
steps_reported() {
  local s
  for s in preflight inputs packages firewall docker layout tls env_files agent first_deploy summary; do
    printf '%s' "$out" | grep -Eq "^==> $s: (done|already done|changed)\$" || { echo "    step $s not reported" >&2; return 1; }
  done
}
steps_already_done_before_deploy() {
  local s
  for s in packages firewall docker layout tls env_files agent; do
    has "==> $s: already done" || { echo "    step $s not 'already done'" >&2; return 1; }
  done
}
# State that a re-run must leave untouched: content, owner and mode.
# One line: it goes into sh -c.
SNAP_PATHS="/opt/awt /etc/nftables.conf /etc/nftables.d"
SNAP_PATHS+=" /etc/systemd/system/awt-agent.service /etc/systemd/system/awt-agent.timer"
SNAP_PATHS+=" /etc/systemd/system/nftables.service.d"
SNAP_PATHS+=" /etc/apt/apt.conf.d/20auto-upgrades /etc/apt/sources.list.d /etc/apt/keyrings /etc/docker"
SNAP_PATHS+=" /home/deploy /etc/passwd /etc/group"
# SSH ports the firewall accepts (the "tcp dport <n> accept" rules), sorted,
# space separated.
ssh_rule_ports() {
  { in_ctr sed -n 's/^[[:space:]]*tcp dport \([0-9]*\) accept$/\1/p' /etc/nftables.d/awt.nft || true; } | sort -n | paste -sd ' ' -
}
ssh_ports_are() {
  local got
  got=$(ssh_rule_ports)
  [ "$got" = "$1" ] || { echo "    SSH rule ports: '$got', want '$1'" >&2; return 1; }
}
not_installed() {
  local p
  for p in "$@"; do
    ! in_ctr dpkg-query -W -f='${db:Status-Abbrev}' "$p" 2>/dev/null | grep -q '^ii' \
      || { echo "    $p is installed" >&2; return 1; }
  done
}
SSH_INPUTS_RE='AWT_ADMIN_|AWT_SSH_PORT'
no_ssh_input() { ! printf '%s' "$out" | grep -Eq "$SSH_INPUTS_RE"; }

snapshot() {
  in_ctr sh -c "find $SNAP_PATHS -exec stat -c '%n %U:%G %a' {} + | sort; find $SNAP_PATHS -type f -exec sha256sum {} + | sort"
}

# nft_parses <ruleset file in $ctr>: the real nft checks the ruleset. With
# NET_ADMIN (a dedicated container, when the Docker daemon allows
# --cap-add) the check is complete; without it nft still parses the whole
# file (syntax errors are reported) and then fails only on its kernel cache,
# which is accepted.
nft_ctr=""
nft_mode=""
nft_parses() {
  local o rc=0
  if [ -z "$nft_mode" ]; then
    if docker run --rm --cap-add NET_ADMIN "$DEBIAN_IMAGE" true >/dev/null 2>&1; then
      nft_mode=privileged
      nft_ctr=$PREFIX-nft
      docker run -d --name "$nft_ctr" --cap-add NET_ADMIN -v "$work:/work:ro" \
        -v "$APT_CACHE:/var/cache/apt/archives" "$DEBIAN_IMAGE" sleep infinity >/dev/null
      ctrs+=("$nft_ctr")
      docker exec "$nft_ctr" sh -c 'rm -f /etc/apt/apt.conf.d/docker-clean; apt-get update -qq >/dev/null && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nftables >/dev/null 2>&1'
    else
      nft_mode=unprivileged
    fi
    echo "    (nft check: $nft_mode)"
  fi
  set +e
  if [ "$nft_mode" = privileged ]; then
    docker exec -i "$nft_ctr" sh -c 'cat > /tmp/check.nft && nft -c -f /tmp/check.nft' < <(in_ctr cat "$1") >"$work/nft.out" 2>&1
  else
    in_ctr /usr/sbin/nft -c -f "$1" >"$work/nft.out" 2>&1
  fi
  rc=$?
  set -e
  o=$(cat "$work/nft.out")
  [ -z "$o" ] || printf '%s\n' "$o" | sed 's/^/    nft| /'
  [ "$rc" = 0 ] && return 0
  [ "$nft_mode" = unprivileged ] && [ -n "$o" ] \
    && ! printf '%s\n' "$o" | grep -Ev 'cache initialization failed|Operation not permitted' | grep -q .
}

echo "== case: every input of the script is documented in docs/ops.md"
for v in $(bash deploy/init-vps.sh --list-inputs); do
  check "docs/ops.md documents $v" grep -qw -- "$v" docs/ops.md
done

echo "== case: no SSH input (SSH access is the operator's setup)"
check "--list-inputs names no AWT_ADMIN_* or AWT_SSH_PORT" eval '! bash deploy/init-vps.sh --list-inputs | grep -Eq "$SSH_INPUTS_RE"'

echo "== case: missing values, non-interactive"
start_ctr cheap
run_init
check "exit non-zero" [ "$rc" != 0 ]
for v in AWT_PROD_HOST AWT_TRAINING_HOST \
  PROD_S3_ENDPOINT PROD_S3_REGION PROD_S3_BUCKET PROD_S3_ACCESS_KEY PROD_S3_SECRET_KEY \
  TRAINING_S3_ENDPOINT TRAINING_S3_REGION TRAINING_S3_BUCKET TRAINING_S3_ACCESS_KEY TRAINING_S3_SECRET_KEY \
  AWT_TLS_CERT_FILE AWT_TLS_KEY_FILE; do
  check "lists $v" has "$v"
done
check "no SSH input listed as missing" no_ssh_input
check "nothing installed (no apt run)" eval '! exists /usr/sbin/nft && ! exists /opt/awt'

echo "== case: invalid inputs, all refused at once before anything changes"
run_init "${FULL[@]}" -e PROD_S3_ENDPOINT=http://s3.gra.io.cloud.ovh.net -e TRAINING_MFA_KEY=not-base64-32 \
  -e PROD_BUCKET_PREFIX=prod/../x -e TRAINING_BUCKET_PREFIX=a//b
check "exit non-zero" [ "$rc" != 0 ]
check "http:// S3 endpoint refused (https only)" has "PROD_S3_ENDPOINT: must be an https:// URL"
check "invalid MFA key named too (all errors at once)" has "TRAINING_MFA_KEY: must be base64 of 32 bytes"
check "bucket prefixes with a .. segment or // refused" eval 'has "PROD_BUCKET_PREFIX: must be empty or segments" && has "TRAINING_BUCKET_PREFIX: must be empty or segments"'
check "nothing installed, no systemctl call" eval '! exists /usr/sbin/nft && ! exists /opt/awt && [ -z "$(calls)" ]'

echo "== case: interactive, only one TLS file given"
run_init_tty $'\n\n\n\n\n\n' "${FULL[@]}" -e AWT_TLS_KEY_FILE=
check "exit non-zero" [ "$rc" != 0 ]
check "says the key file is required with the certificate" has "AWT_TLS_KEY_FILE: required with AWT_TLS_CERT_FILE"
check "does not claim stdin is not a terminal" hasnt "stdin is not a terminal"
check "no SSH input prompted (AWT_ADMIN_*, AWT_SSH_PORT)" no_ssh_input
check "nothing changed" eval '! exists /usr/sbin/nft && ! exists /opt/awt'

echo "== case: not Debian 13"
in_ctr sh -c 'printf "ID=ubuntu\nVERSION_ID=\"24.04\"\n" > /etc/os-release.test && mv /etc/os-release /etc/os-release.orig && cp /etc/os-release.test /etc/os-release'
set +e
out=$(docker exec -i -e AWT_REPO_RAW=file:///src "$ctr" bash /src/deploy/init-vps.sh </dev/null 2>&1); rc=$?
set -e
check "refused" eval '[ "$rc" != 0 ] && has "Debian 13"'
in_ctr mv /etc/os-release.orig /etc/os-release

echo "== case: not root"
set +e
out=$(docker exec -i -u 65534 "${BASE_ENV[@]}" "$ctr" bash /src/deploy/init-vps.sh </dev/null 2>&1); rc=$?
set -e
check "refused" eval '[ "$rc" != 0 ] && has "root"'

echo "== case: happy path (fresh Debian 13, everything given, no replica)"
start_ctr happy
run_init "${FULL[@]}"
check "exit 0" [ "$rc" = 0 ]
check "every step reported" steps_reported
check "no SSH input required (exit 0 without AWT_ADMIN_*, AWT_SSH_PORT)" eval '[ "$rc" = 0 ] && no_ssh_input'
check "packages installed" in_ctr dpkg-query -W nftables unattended-upgrades docker-ce docker-compose-plugin
check "no SSH setup: no openssh-server, sudo, fail2ban installed" not_installed openssh-server sudo fail2ban python3-systemd
check "no SSH setup: no sshd drop-in, sudoers file or fail2ban jail" eval '! exists /etc/ssh/sshd_config.d/10-awt-hardening.conf && ! exists /etc/sudoers.d/90-awt-admin && ! exists /etc/fail2ban/jail.d/awt-sshd.local'
check "docker apt source" file_has /etc/apt/sources.list.d/docker.sources 'Signed-By: /etc/apt/keyrings/docker.asc'
check "unattended upgrades on" file_has /etc/apt/apt.conf.d/20auto-upgrades 'Unattended-Upgrade "1"'
check "no sshd, not run over SSH: SSH rule on 22 only" ssh_ports_are 22
check "nftables: cloudflare sets" file_has /etc/nftables.d/awt.nft '173\.245\.48\.0/20'
check "nftables: docker-published ports filtered (forward hook)" file_has /etc/nftables.d/awt.nft 'ct status dnat'
check "nftables.conf includes, no flush ruleset" eval 'file_has /etc/nftables.conf "include \"/etc/nftables.d/\*.nft\"" && ! file_has /etc/nftables.conf "^[[:space:]]*flush ruleset"'
check "nftables loaded and enabled" eval 'calls | grep -q "^nft -f /etc/nftables.conf" && calls | grep -q "enable.*nftables"'
check "first load in two phases: policy accept, then the drop ruleset" eval 'in_ctr grep -q "policy accept;" /var/lib/awt-test/nft-load.1 && ! in_ctr grep -q "policy drop;" /var/lib/awt-test/nft-load.1 && in_ctr grep -q "include" /var/lib/awt-test/nft-load.2 && [ "$(calls | grep "^nft -f" | tail -1)" = "nft -f /etc/nftables.conf" ]'
check "generated ruleset parsed by the real nft" nft_parses /etc/nftables.d/awt.nft
in_ctr sh -c 'sed "s/tcp dport 22 accept/tcp dprt 22 accept/" /etc/nftables.d/awt.nft > /tmp/broken.nft'
check "the nft check catches a broken ruleset" eval '! nft_parses /tmp/broken.nft'
check "deploy user: docker group, no login, no key" eval 'in_ctr id -nG deploy | grep -qw docker && [ "$(in_ctr getent passwd deploy | cut -d: -f7)" = /usr/sbin/nologin ] && ! exists /home/deploy/.ssh/authorized_keys'
check "layout owners/modes" eval '[ "$(stat_of /opt/awt)" = "deploy:deploy 750" ] && [ "$(stat_of /opt/awt/state)" = "deploy:deploy 750" ] && [ "$(stat_of /opt/awt/caddy)" = "deploy:deploy 755" ] && [ "$(stat_of /opt/awt/certs)" = "deploy:deploy 700" ] && [ "$(stat_of /opt/awt/deploy)" = "root:root 755" ]'
for f in agent.sh deploy.sh weekly-restore-test.sh; do
  check "deploy/$f installed root 755, from the repo" eval '[ "$(stat_of /opt/awt/deploy/$f)" = "root:root 755" ] && same_as_repo deploy/$f /opt/awt/deploy/$f'
done
for u in awt-agent.service awt-agent.timer; do
  check "$u installed from the repo" eval '[ "$(stat_of /etc/systemd/system/$u)" = "root:root 644" ] && same_as_repo deploy/$u /etc/systemd/system/$u'
done
check "timer enabled and started" eval 'calls | grep -Eq "^systemctl enable (--now awt-agent.timer|awt-agent.timer --now)"'
check "certs installed" eval '[ "$(stat_of /opt/awt/certs/origin.pem)" = "deploy:deploy 644" ] && [ "$(stat_of /opt/awt/certs/origin.key)" = "deploy:deploy 600" ] && in_ctr cmp -s /work/origin.key /opt/awt/certs/origin.key'
for f in .env .env.prod .env.training; do
  check "$f mode 600 owner deploy" [ "$(stat_of /opt/awt/$f)" = "deploy:deploy 600" ]
done
check ".env has every key" all_keys env.example /opt/awt/.env
check ".env.prod has every key" all_keys env.prod.example /opt/awt/.env.prod
check ".env.training has every key" all_keys env.training.example /opt/awt/.env.training
check ".env values" eval '[ "$(env_value /opt/awt/.env AWT_IMAGE)" = ghcr.io/fatpat/awt ] && [ "$(env_value /opt/awt/.env AWT_CHANNEL)" = rewrite-go ] && [ "$(env_value /opt/awt/.env AWT_PROD_HOST)" = prod.example.org ] && [ "$(env_value /opt/awt/.env AWT_TRAINING_HOST)" = training.example.org ]'
check ".env.prod values" eval '[ "$(env_value /opt/awt/.env.prod AWT_BASE_URL)" = https://prod.example.org ] && [ "$(env_value /opt/awt/.env.prod AWT_S3_BUCKET)" = awt-prod ] && [ "$(env_value /opt/awt/.env.prod AWT_S3_SECRET_KEY)" = prod-secret-value-9876 ] && [ "$(env_value /opt/awt/.env.prod LITESTREAM_S3_REGION)" = gra ] && [ "$(env_value /opt/awt/.env.prod AWT_CLOUDFLARE_TOKEN)" = cf-token-value-5555 ]'
check ".env.training values" eval '[ "$(env_value /opt/awt/.env.training AWT_BASE_URL)" = https://training.example.org ] && [ "$(env_value /opt/awt/.env.training AWT_S3_BUCKET)" = awt-training ] && [ "$(env_value /opt/awt/.env.training AWT_S3_ACCESS_KEY)" = trainaccess0002 ]'
check "default bucket prefixes prod/ and training/" eval '[ "$(env_value /opt/awt/.env.prod AWT_BUCKET_PREFIX)" = prod/ ] && [ "$(env_value /opt/awt/.env.training AWT_BUCKET_PREFIX)" = training/ ] && has "prefix prod/" && has "prefix training/"'
# shellcheck disable=SC2034 # used in eval checks
mfa_prod=$(env_value /opt/awt/.env.prod AWT_MFA_KEY)
# shellcheck disable=SC2034
mfa_training=$(env_value /opt/awt/.env.training AWT_MFA_KEY)
check "MFA keys generated (no replica), base64 32 bytes, distinct" eval 'b64_32 "$mfa_prod" && b64_32 "$mfa_training" && [ "$mfa_prod" != "$mfa_training" ]'
check "replica checked for both environments, under their prefix" eval 'in_ctr grep -q "AWT_BUCKET_PREFIX=prod/ .*awt bucket-path litestream.*snapshots" /var/lib/awt-test/docker.log && in_ctr grep -q "AWT_BUCKET_PREFIX=training/ .*awt bucket-path litestream.*snapshots" /var/lib/awt-test/docker.log'
check "secrets handled under umask 077" eval '[ -n "$(in_ctr cat /var/lib/awt-test/umask.log)" ] && ! in_ctr grep -vqx 0077 /var/lib/awt-test/umask.log'
check "no temporary file left in /tmp or /opt/awt" eval '[ -z "$(in_ctr find /tmp /opt/awt -maxdepth 1 \( -name "tmp.*" -o -name ".awt-init.*" -o -name "*.tmp" -o -name ".env*.??????" \) -print)" ]'
check "agent lock usable by the agent (deploy)" [ "$(stat_of /run/lock/awt-agent.lock)" = "deploy:deploy 644" ]
check "no secret on a docker command line" eval '! in_ctr grep -Eq "prod-secret-value|training-secret-value" /var/lib/awt-test/docker.log'
check "secrets not printed" eval 'hasnt prod-secret-value-9876 && hasnt training-secret-value-4321 && hasnt cf-token-value-5555 && hasnt "$mfa_prod" && hasnt "$mfa_training" && hasnt prodaccess0001'
check "secrets masked in the summary" eval 'has "****9876" && has "****4321" && has "****5555" && has "****${mfa_prod: -4}"'
check "agent run once, tags deployed" eval '[ "$(agent_runs)" = 1 ] && [ "$(env_value /opt/awt/.env AWT_PROD_TAG)" = "$REV" ] && [ "$(env_value /opt/awt/.env AWT_TRAINING_TAG)" = "$REV" ]'
check "both apps checked through the stack" eval 'in_ctr grep -q "exec -T awt-prod wget" /var/lib/awt-test/docker.log && in_ctr grep -q "exec -T caddy wget.*/training/healthz" /var/lib/awt-test/docker.log'
check "empty database: points to ops.md section 4" eval 'has "empty" && has "docs/ops.md" && has "awt import --from" && has "awt user add"'

echo "== case: re-run on the configured host, no input given"
# shellcheck disable=SC2034
before=$(snapshot)
run_init
check "exit 0" [ "$rc" = 0 ]
check "every step already done, nothing changed" steps_already_done
check "files, owners and modes unchanged" eval '[ "$(printf "%s\n" "$before" | wc -l)" -gt 50 ] && [ "$before" = "$(snapshot)" ]'
check "agent not run again" [ "$(agent_runs)" = 1 ]
check "no prompt / missing value" hasnt "missing"

echo "== case: sshd -T reports port 22, not run over SSH"
in_ctr sh -c 'install -m 755 /work/sshd-stub /usr/local/sbin/sshd; echo "port 22" > /var/lib/awt-test/sshd-T; : > /var/lib/awt-test/calls.log'
run_init
check "exit 0" [ "$rc" = 0 ]
check "sshd -T read" eval 'calls | grep -qx "sshd -T"'
check "SSH rule on 22 only" ssh_ports_are 22
check "firewall already done (same rule as the fallback)" has "==> firewall: already done"

echo "== case: sshd moved to 2200, run over SSH on port 22, firewall already up, AWT_INIT_FORCE=1"
in_ctr sh -c 'echo "port 2200" > /var/lib/awt-test/sshd-T; : > /var/lib/awt-test/calls.log; rm -f /var/lib/awt-test/nft-load.*'
run_init "${FULL[@]}" -e AWT_INIT_FORCE=1 -e "SSH_CONNECTION=203.0.113.5 50022 198.51.100.7 22" \
  -e PROD_BUCKET_PREFIX=prod -e TRAINING_BUCKET_PREFIX=
check "exit 0" [ "$rc" = 0 ]
check "bucket prefix: prod normalised to prod/, training emptied (dedicated bucket)" eval 'grep -qx "AWT_BUCKET_PREFIX=prod/" <<<"$(in_ctr cat /opt/awt/.env.prod)" && grep -qx "AWT_BUCKET_PREFIX=" <<<"$(in_ctr cat /opt/awt/.env.training)" && has "prefix (none)"'
check "SSH rules: 2200 (sshd -T) and 22 (this session)" ssh_ports_are "22 2200"
check "says this session's port is kept" has "port 22 (this SSH session)"
check "table already up: loaded once, no policy-accept phase" eval '[ "$(calls | grep -c "^nft -f")" = 1 ] && ! in_ctr grep -q "policy accept;" /var/lib/awt-test/nft-load.1'
check "ruleset with two SSH ports parsed by the real nft" nft_parses /etc/nftables.d/awt.nft
check "sshd never reloaded or restarted" eval '! calls | grep -Eq "^systemctl .*ssh"'
check "installed certificate kept" in_ctr cmp -s /work/origin.pem /opt/awt/certs/origin.pem

echo "== case: re-run from a session on 2200, AWT_INIT_FORCE=1, no input: port 22 closed"
run_init -e AWT_INIT_FORCE=1 -e "SSH_CONNECTION=203.0.113.5 50023 198.51.100.7 2200"
check "exit 0" [ "$rc" = 0 ]
check "no certificate asked for (installed one kept)" eval 'hasnt AWT_TLS_CERT_FILE && hasnt Missing'
check "SSH rule on 2200 only" ssh_ports_are 2200
check "empty training prefix kept on a re-run without input" eval 'grep -qx "AWT_BUCKET_PREFIX=" <<<"$(in_ctr cat /opt/awt/.env.training)"'
check "firewall changed" has "==> firewall: changed"

echo "== case: sshd -T reports several ports"
in_ctr sh -c 'printf "port 2200\nport 22\n" > /var/lib/awt-test/sshd-T'
run_init
check "exit 0" [ "$rc" = 0 ]
check "SSH rules on 22 and 2200" ssh_ports_are "22 2200"

echo "== case: sshd -T fails: 22, plus this session's port"
in_ctr sh -c 'touch /var/lib/awt-test/sshd-fail; : > /var/lib/awt-test/calls.log'
run_init -e "SSH_CONNECTION=203.0.113.5 50024 198.51.100.7 2200"
check "exit 0" [ "$rc" = 0 ]
check "sshd -T tried" eval 'calls | grep -qx "sshd -T"'
check "SSH rules: 22 (fallback) and 2200 (this session)" ssh_ports_are "22 2200"

echo "== case: certificate and key do not match"
start_ctr resume
run_init "${FULL[@]}" -e AWT_TLS_KEY_FILE=/work/other.key
check "exit non-zero" [ "$rc" != 0 ]
check "says they do not match" has "do not match"
check "no certificate installed" eval '! exists /opt/awt/certs/origin.pem && ! exists /opt/awt/certs/origin.key'

echo "== case: litestream reports an S3 error but exits 0"
in_ctr touch /var/lib/awt-test/s3-error
run_init "${FULL[@]}"
check "exit non-zero" [ "$rc" != 0 ]
check "reports the S3 error" eval 'has "cannot list s3://awt-prod/prod/litestream/" && has "cannot determine snapshots"'
check "no MFA key generated, .env.prod not written" eval 'hasnt "generated PROD_MFA_KEY" && ! exists /opt/awt/.env.prod'
in_ctr rm /var/lib/awt-test/s3-error

echo "== case: S3 holds a prod replica, no PROD_MFA_KEY, non-interactive"
in_ctr touch /var/lib/awt-test/replica-prod
run_init "${FULL[@]}"
check "exit non-zero" [ "$rc" != 0 ]
check "asks for PROD_MFA_KEY and explains" eval 'has PROD_MFA_KEY && has replica'
check "no key generated, .env.prod not written" eval '! exists /opt/awt/.env.prod'

echo "== case: same, interactive: prompts for the key; first deploy fails"
in_ctr touch /var/lib/awt-test/agent-fail
run_init_tty "$MFA_GIVEN"$'\n' "${FULL[@]}" -e TRAINING_CLOUDFLARE_ZONE_ID= -e TRAINING_CLOUDFLARE_TOKEN=
check "exit non-zero" [ "$rc" != 0 ]
check "prompted for the prod MFA key with a warning" eval 'has PROD_MFA_KEY && has replica'
check "given key stored" [ "$(env_value /opt/awt/.env.prod AWT_MFA_KEY)" = "$MFA_GIVEN" ]
check "training key generated" eval 'b64_32 "$(env_value /opt/awt/.env.training AWT_MFA_KEY)"'
check "agent journal tail printed" has "simulated failure"
check "next steps printed" eval 'has "re-run" && has "failed-revisions"'
check "agent and units stay installed" eval 'exists /opt/awt/deploy/agent.sh && exists /etc/systemd/system/awt-agent.timer && calls | grep -Eq "^systemctl enable (--now awt-agent.timer|awt-agent.timer --now)"'

echo "== case: re-run after the failure resumes (no input given)"
in_ctr rm /var/lib/awt-test/agent-fail
run_init
check "exit 0" [ "$rc" = 0 ]
check "agent run again (2 runs in all)" [ "$(agent_runs)" = 2 ]
check "steps before the deploy already done" steps_already_done_before_deploy
check "prod restored from the S3 replica" eval 'printf "%s" "$out" | grep -Eiq "prod.*restored"'
check "training empty: points to ops.md section 4" eval 'printf "%s" "$out" | grep -Eiq "training.*empty" && has "awt import --from"'

if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures check(s) failed" >&2
  exit 1
fi
echo "PASS: VPS init"
