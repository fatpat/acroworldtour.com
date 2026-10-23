#!/usr/bin/env bash
# VPS initialization: turns a fresh Debian 13 (trixie) server into a running
# AcroWorldTour host (prod + training), ending after the first deploy.
# Automates docs/ops.md sections 1-3 and 5; see docs/ops.md section 0.
#
#   curl -fsSLO https://raw.githubusercontent.com/fatpat/acroworldtour.com/rewrite-go/deploy/init-vps.sh
#   less init-vps.sh                              # review it first
#   sudo AWT_ADMIN_USER=... ... bash init-vps.sh  # or answer the prompts
#
# Every input comes from an environment variable or, when unset and stdin is a
# terminal, from a prompt (secrets hidden). Without a terminal, all missing
# values are listed at once and nothing is changed. `bash init-vps.sh
# --list-inputs` prints the input names.
#
# Idempotent: each step checks before acting and reports
# "==> <step>: changed|already done". Values already stored on the host (env
# files, sshd configuration, certificates) are reused and never overwritten
# unless AWT_INIT_FORCE=1. A failed run can simply be run again: it resumes.
#
# Repository files (agent, deploy scripts, systemd units, env examples) are
# downloaded from $AWT_REPO_RAW; compose.yaml and the Caddyfile come from the
# image (the deploy agent installs them). The first deploy is one run of the
# agent: if the S3 bucket holds a Litestream replica the container restores it
# at start (deploy/entrypoint.sh), otherwise the database starts empty.
set -Eeuo pipefail
shopt -s inherit_errexit
umask 022

# Input names (docs/ops.md section 0 documents the same names).
GLOBAL_INPUTS=(AWT_ADMIN_USER AWT_ADMIN_SSH_KEY AWT_SSH_PORT AWT_PROD_HOST AWT_TRAINING_HOST
  AWT_IMAGE AWT_CHANNEL AWT_REPO_RAW AWT_TLS_CERT_FILE AWT_TLS_KEY_FILE)
ENV_INPUTS=(S3_ENDPOINT S3_REGION S3_BUCKET S3_ACCESS_KEY S3_SECRET_KEY MFA_KEY CLOUDFLARE_ZONE_ID CLOUDFLARE_TOKEN)
CONTROL_INPUTS=(AWT_INIT_FORCE AWT_INIT_ALLOW_OS)
ENVS=(prod training)

if [ "${1:-}" = --list-inputs ]; then
  printf '%s\n' "${GLOBAL_INPUTS[@]}"
  for e in PROD TRAINING; do for n in "${ENV_INPUTS[@]}"; do echo "${e}_$n"; done; done
  printf '%s\n' "${CONTROL_INPUTS[@]}"
  exit 0
fi

ROOT=/opt/awt
SSHD_CONF=/etc/ssh/sshd_config.d/10-awt-hardening.conf
NFT_FILE=/etc/nftables.d/awt.nft
F2B_JAIL=/etc/fail2ban/jail.d/awt-sshd.local
SUDOERS=/etc/sudoers.d/90-awt-admin
DOCKER_KEY=/etc/apt/keyrings/docker.asc
# Docker's release signing key (https://docs.docker.com/engine/install/debian/).
DOCKER_KEY_FPR=9DC858229FC7DD38854AE2D88D81803C0EBFCD88
FORCE=${AWT_INIT_FORCE:-0}
# Fallback when https://www.cloudflare.com/ips-v4 and ips-v6 cannot be fetched
# (same list as the Caddyfile).
CF4_FALLBACK="173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22"
CF6_FALLBACK="2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32"

declare -A DESC=(
  [AWT_ADMIN_USER]="admin login (SSH, sudo)"
  [AWT_ADMIN_SSH_KEY]="admin SSH public key (one authorized_keys line)"
  [AWT_SSH_PORT]="SSH port (1024-65535)"
  [AWT_PROD_HOST]="prod site host name"
  [AWT_TRAINING_HOST]="training site host name"
  [AWT_IMAGE]="image name"
  [AWT_CHANNEL]="moving tag the agent follows"
  [AWT_REPO_RAW]="base URL of the repository files"
  [AWT_TLS_CERT_FILE]="Cloudflare Origin CA certificate (PEM file)"
  [AWT_TLS_KEY_FILE]="its private key (PEM file)"
  [S3_ENDPOINT]="S3 endpoint URL"
  [S3_REGION]="S3 region"
  [S3_BUCKET]="S3 bucket (Litestream replica, uploads)"
  [S3_ACCESS_KEY]="S3 access key"
  [S3_SECRET_KEY]="S3 secret key"
  [MFA_KEY]="manager MFA key (base64, 32 bytes)"
  [CLOUDFLARE_ZONE_ID]="Cloudflare zone id (optional, with the token)"
  [CLOUDFLARE_TOKEN]="Cloudflare cache purge token (optional)"
)

INTERACTIVE=0
[ -t 0 ] && INTERACTIVE=1
current_step=""
step_changed=0
missing=()
errors=()
new_inputs=0

log() { printf '%s\n' "$*"; }
note() { printf '    %s\n' "$*"; }
warn() { printf 'WARNING: %s\n' "$*" >&2; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
changed() { step_changed=1; [ $# -eq 0 ] || note "$*"; }
mask() { if [ -z "${1:-}" ]; then echo "(not set)"; elif [ "${#1}" -le 8 ]; then echo "****"; else echo "****${1: -4}"; fi; }

on_err() { printf 'ERROR: command failed (exit %s) at line %s: %s\n' "$1" "$2" "$3" >&2; }
on_exit() {
  local rc=$?
  if [ "$rc" != 0 ] && [ -n "$current_step" ]; then
    printf '==> %s: FAILED. Fix the cause above, then re-run this script to resume (completed steps are skipped).\n' "$current_step" >&2
  fi
}
trap 'on_err $? $LINENO "$BASH_COMMAND"' ERR
trap on_exit EXIT

# run_step <name>: runs step_<name> and reports whether it changed anything.
run_step() {
  current_step=$1
  step_changed=0
  "step_$1"
  if [ "$step_changed" = 1 ]; then log "==> $1: changed"; else log "==> $1: already done"; fi
  current_step=""
}

# put_file <path> <owner:group> <mode> < content: replaces the file only when
# its content, owner or mode differ (written next to it, then renamed).
put_file() {
  local dest=$1 own=$2 mode=$3 tmp
  tmp=$(mktemp "$(dirname "$dest")/.awt-init.XXXXXX")
  cat > "$tmp"
  if [ -f "$dest" ] && cmp -s "$tmp" "$dest"; then
    rm -f "$tmp"
    if [ "$(stat -c '%U:%G %a' "$dest")" != "$own $mode" ]; then
      chown "$own" "$dest"; chmod "$mode" "$dest"
      changed "set $dest to $own $mode"
    fi
    return 0
  fi
  chown "$own" "$tmp"; chmod "$mode" "$tmp"
  mv -f "$tmp" "$dest"
  changed "wrote $dest"
}

# ensure_dir <path> <owner:group> <mode>
ensure_dir() {
  if [ ! -d "$1" ]; then
    install -d -o "${2%:*}" -g "${2#*:}" -m "$3" "$1"
    changed "created $1"
  elif [ "$(stat -c '%U:%G %a' "$1")" != "$2 $3" ]; then
    chown "$2" "$1"; chmod "$3" "$1"
    changed "set $1 to $2 $3"
  fi
}

# fetch <repository path> <local file>: downloads a file of the repository.
fetch() {
  local url="${V_AWT_REPO_RAW%/}/$1"
  curl -fsSL --proto '=https,file' --retry 3 --max-time 60 -o "$2" "$url" \
    || die "cannot download $url (check AWT_REPO_RAW / AWT_CHANNEL)"
  [ -s "$2" ] || die "$url is empty"
}

# file_value <file> <key>: last value of KEY= in an env file (empty if none).
file_value() {
  [ -r "$1" ] || return 0
  sed -n "s/^$2=//p" "$1" | tail -1
}
file_has_key() { [ -r "$1" ] && grep -q "^$2=" "$1"; }

# set_key <file> <key> <value>: replaces every KEY= line, or appends one.
set_key() {
  K=$2 V=$3 awk 'BEGIN { k = ENVIRON["K"]; v = ENVIRON["V"] }
    index($0, k "=") == 1 { print k "=" v; done = 1; next } { print }
    END { if (!done) print k "=" v }' "$1" > "$1.tmp"
  mv -f "$1.tmp" "$1"
}

dc() { (cd "$ROOT" && docker compose "$@"); }

# ---------------------------------------------------------------- validation

# check_value <kind> <value>: prints the problem, returns 1, when invalid.
check_value() {
  local kind=$1 v=$2
  case "$kind" in
    user)
      [[ $v =~ ^[a-z_][a-z0-9_-]{0,31}$ ]] || { echo "must be a lowercase login name"; return 1; }
      case "$v" in root|deploy) echo "must not be root or deploy"; return 1 ;; esac ;;
    sshkey) valid_ssh_key "$v" || { echo "must be one SSH public key line (type, base64 key, optional comment)"; return 1; } ;;
    port)
      [[ $v =~ ^[0-9]{1,5}$ ]] && [ "$((10#$v))" -ge 1024 ] && [ "$((10#$v))" -le 65535 ] \
        || { echo "must be a port number from 1024 to 65535"; return 1; } ;;
    host)
      [[ $v =~ ^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,61}[a-z0-9]$ ]] \
        || { echo "must be a lowercase host name (www.example.org)"; return 1; } ;;
    image) [[ $v =~ ^[a-z0-9][a-z0-9._/:-]*[a-z0-9]$ ]] || { echo "must be an image name without tag (ghcr.io/owner/awt)"; return 1; } ;;
    tag) [[ $v =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || { echo "must be an image tag"; return 1; } ;;
    rawurl) [[ $v =~ ^(https|file)://[^[:space:]]+$ ]] || { echo "must be an https:// URL"; return 1; } ;;
    url) [[ $v =~ ^https?://[A-Za-z0-9.-]+(:[0-9]+)?/?$ ]] || { echo "must be an URL like https://s3.gra.io.cloud.ovh.net"; return 1; } ;;
    region) [[ $v =~ ^[a-z0-9-]{1,32}$ ]] || { echo "must be a region name like gra"; return 1; } ;;
    bucket) [[ $v =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || { echo "must be an S3 bucket name"; return 1; } ;;
    token)
      # Written as KEY=value in env files read by compose: no quoting needed.
      case "$v" in ""|*[[:space:]\"\'\`\$\#\\]*) echo "must be non-empty without spaces, quotes, \$, # or backslash"; return 1 ;; esac ;;
    mfa)
      [[ $v =~ ^[A-Za-z0-9+/]+={0,2}$ ]] && [ "$(printf '%s' "$v" | base64 -d 2>/dev/null | wc -c)" = 32 ] \
        || { echo "must be base64 of 32 bytes (openssl rand -base64 32)"; return 1; } ;;
    zone) [[ $v =~ ^[0-9a-f]{32}$ ]] || { echo "must be a 32-character hexadecimal Cloudflare zone id"; return 1; } ;;
    pemfile)
      [ -r "$v" ] || { echo "file $v is not readable"; return 1; }
      grep -q -- '-----BEGIN ' "$v" || { echo "$v is not a PEM file"; return 1; } ;;
  esac
}

# The base64 blob of an OpenSSH public key starts with its length-prefixed type.
valid_ssh_key() {
  local type blob rest decoded
  [[ $1 != *$'\n'* ]] || return 1
  read -r type blob rest <<<"$1"
  case "$type" in
    ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp256|ecdsa-sha2-nistp384|ecdsa-sha2-nistp521|sk-ssh-ed25519@openssh.com|sk-ecdsa-sha2-nistp256@openssh.com) ;;
    *) return 1 ;;
  esac
  [[ $blob =~ ^[A-Za-z0-9+/]+={0,2}$ ]] || return 1
  decoded=$({ printf '%s' "$blob" | base64 -d 2>/dev/null || true; } | head -c $((4 + ${#type})) | tail -c "${#type}")
  [ "$decoded" = "$type" ]
}

kind_of() {
  case "$1" in
    AWT_ADMIN_USER) echo user ;; AWT_ADMIN_SSH_KEY) echo sshkey ;; AWT_SSH_PORT) echo port ;;
    *_HOST) echo host ;; AWT_IMAGE) echo image ;; AWT_CHANNEL) echo tag ;; AWT_REPO_RAW) echo rawurl ;;
    AWT_TLS_*_FILE) echo pemfile ;; *_S3_ENDPOINT) echo url ;; *_S3_REGION) echo region ;;
    *_S3_BUCKET) echo bucket ;; *_MFA_KEY) echo mfa ;; *_CLOUDFLARE_ZONE_ID) echo zone ;; *) echo token ;;
  esac
}
desc_of() { local n=$1; n=${n#PROD_}; n=${n#TRAINING_}; echo "${DESC[$1]:-${DESC[$n]:-}}"; }

# ask <name> <secret 0|1> <optional 0|1> [default]: prompts until valid.
ask() {
  local name=$1 secret=$2 optional=$3 default=${4:-} v msg tries=0 hint=""
  [ -z "$default" ] || hint=" [$default]"
  [ "$optional" = 0 ] || hint=" (optional, Enter to skip)"
  while :; do
    if [ "$secret" = 1 ]; then
      read -rs -p "$name - $(desc_of "$name")$hint: " v; echo >&2
    else
      read -r -p "$name - $(desc_of "$name")$hint: " v
    fi
    [ -n "$v" ] || v=$default
    if [ -z "$v" ] && [ "$optional" = 1 ]; then printf -v "V_$name" '%s' ""; return 0; fi
    if [ -n "$v" ] && msg=$(check_value "$(kind_of "$name")" "$v"); then
      printf -v "V_$name" '%s' "$v"; return 0
    fi
    echo "  $name ${msg:-is required}" >&2
    tries=$((tries + 1))
    [ "$tries" -lt 5 ] || die "$name: too many invalid answers"
  done
}

# resolve <name> <stored value> <required|optional|default:<value>> <secret 0|1> [ask optional 0|1]
# Order: stored value (kept unless AWT_INIT_FORCE=1), environment, prompt,
# default. Missing required values are collected, invalid ones too, so a
# non-interactive run reports them all at once.
resolve() {
  local name=$1 stored=$2 mode=$3 secret=$4 ask_optional=${5:-0} given="" isset=0 msg
  if [ -n "${!name+x}" ]; then isset=1; given=${!name}; fi
  if [ -n "$stored" ]; then
    printf -v "V_$name" '%s' "$stored"
    if [ -n "$given" ] && [ "$given" != "$stored" ]; then
      if [ "$FORCE" = 1 ]; then
        printf -v "V_$name" '%s' "$given"; new_inputs=1
      else
        warn "$name: keeping the value already stored on this host (AWT_INIT_FORCE=1 replaces it)"
      fi
    fi
  elif [ -n "$given" ]; then
    printf -v "V_$name" '%s' "$given"; new_inputs=1
  elif [ "$mode" = optional ]; then
    if [ "$INTERACTIVE" = 1 ] && [ "$isset" = 0 ] && [ "$ask_optional" = 1 ]; then
      ask "$name" "$secret" 1; new_inputs=1
    else
      printf -v "V_$name" '%s' ""
    fi
    return 0
  elif [[ $mode == default:* ]]; then
    if [ "$INTERACTIVE" = 1 ] && [ "$name" = AWT_SSH_PORT ]; then
      ask "$name" "$secret" 0 "${mode#default:}"; new_inputs=1; return 0
    fi
    printf -v "V_$name" '%s' "${mode#default:}"
  elif [ "$INTERACTIVE" = 1 ]; then
    ask "$name" "$secret" 0; new_inputs=1
    return 0
  else
    printf -v "V_$name" '%s' ""
    missing+=("$name")
    return 0
  fi
  local val="V_$name"
  if [ -n "${!val}" ] && ! msg=$(check_value "$(kind_of "$name")" "${!val}"); then
    errors+=("$name: $msg")
  fi
}

# read_pem <label> <secret 0|1>: file path or pasted PEM (until the END line).
read_pem() {
  local first line pem="" ropt=(-r)
  [ "$2" = 0 ] || ropt=(-r -s)
  read "${ropt[@]}" -p "$1 - file path, or paste the PEM: " first
  [ "$2" = 0 ] || echo >&2
  if [[ $first == -----BEGIN* ]]; then
    pem=$first
    while IFS= read "${ropt[@]}" line; do
      pem+=$'\n'$line
      [[ $line != -----END* ]] || break
    done
    [ "$2" = 0 ] || echo >&2
  else
    check_value pemfile "$first" >&2 || die "$1: cannot read $first"
    pem=$(cat "$first")
  fi
  printf '%s' "$pem"
}

# ---------------------------------------------------------------- steps

step_preflight() {
  [ "$(id -u)" = 0 ] || die "must run as root (sudo bash init-vps.sh)"
  local id version pretty
  id=$(. /etc/os-release && echo "${ID:-}")
  version=$(. /etc/os-release && echo "${VERSION_ID:-}")
  pretty=$(. /etc/os-release && echo "${PRETTY_NAME:-$id $version}")
  if [ "$id" != debian ] || [ "$version" != 13 ]; then
    [ "${AWT_INIT_ALLOW_OS:-0}" = 1 ] || die "this script supports Debian 13 (trixie) only, found: $pretty"
    warn "not Debian 13 ($pretty): continuing because AWT_INIT_ALLOW_OS=1"
  fi
  [ "$INTERACTIVE" = 1 ] || note "stdin is not a terminal: no prompts, every value comes from the environment"
}

step_inputs() {
  local e E f home stored_user stored_port
  stored_user=""; stored_port=""
  if [ -r "$SSHD_CONF" ]; then
    stored_user=$(sed -n 's/^AllowUsers //p' "$SSHD_CONF" | head -1)
    stored_port=$(sed -n 's/^Port //p' "$SSHD_CONF" | head -1)
  fi
  resolve AWT_ADMIN_USER "$stored_user" required 0
  # The key is needed only while the admin account has none.
  home=""
  if [ -n "$V_AWT_ADMIN_USER" ] && id -u "$V_AWT_ADMIN_USER" >/dev/null 2>&1; then
    home=$(getent passwd "$V_AWT_ADMIN_USER" | cut -d: -f6)
  fi
  if [ -n "$home" ] && grep -Eq '^(ssh-|ecdsa-|sk-)' "$home/.ssh/authorized_keys" 2>/dev/null; then
    if [ -n "${AWT_ADMIN_SSH_KEY:-}" ]; then resolve AWT_ADMIN_SSH_KEY "" required 0; else V_AWT_ADMIN_SSH_KEY=""; fi
  else
    resolve AWT_ADMIN_SSH_KEY "" required 0
  fi
  resolve AWT_SSH_PORT "$stored_port" default:2222 0
  resolve AWT_PROD_HOST "$(file_value "$ROOT/.env" AWT_PROD_HOST)" required 0
  resolve AWT_TRAINING_HOST "$(file_value "$ROOT/.env" AWT_TRAINING_HOST)" required 0
  resolve AWT_IMAGE "$(file_value "$ROOT/.env" AWT_IMAGE)" default:ghcr.io/fatpat/awt 0
  resolve AWT_CHANNEL "$(file_value "$ROOT/.env" AWT_CHANNEL)" default:rewrite-go 0
  resolve AWT_REPO_RAW "" "default:https://raw.githubusercontent.com/fatpat/acroworldtour.com/$V_AWT_CHANNEL" 0

  for e in "${ENVS[@]}"; do
    E=${e^^}; f="$ROOT/.env.$e"
    resolve "${E}_S3_ENDPOINT" "$(file_value "$f" AWT_S3_ENDPOINT)" required 0
    resolve "${E}_S3_REGION" "$(file_value "$f" LITESTREAM_S3_REGION)" required 0
    resolve "${E}_S3_BUCKET" "$(file_value "$f" AWT_S3_BUCKET)" required 0
    resolve "${E}_S3_ACCESS_KEY" "$(file_value "$f" AWT_S3_ACCESS_KEY)" required 1
    resolve "${E}_S3_SECRET_KEY" "$(file_value "$f" AWT_S3_SECRET_KEY)" required 1
    # Generated or required later (step env_files), depending on the replica.
    resolve "${E}_MFA_KEY" "$(file_value "$f" AWT_MFA_KEY)" optional 1 0
    # Optional: asked only before the env file exists.
    local ask_cf=1
    [ ! -f "$f" ] || ask_cf=0
    resolve "${E}_CLOUDFLARE_ZONE_ID" "$(file_value "$f" AWT_CLOUDFLARE_ZONE_ID)" optional 0 "$ask_cf"
    local zone="V_${E}_CLOUDFLARE_ZONE_ID"
    if [ -n "${!zone}" ]; then
      resolve "${E}_CLOUDFLARE_TOKEN" "$(file_value "$f" AWT_CLOUDFLARE_TOKEN)" required 1
    else
      resolve "${E}_CLOUDFLARE_TOKEN" "$(file_value "$f" AWT_CLOUDFLARE_TOKEN)" optional 1 0
      local tok="V_${E}_CLOUDFLARE_TOKEN"
      [ -z "${!tok}" ] || errors+=("${E}_CLOUDFLARE_ZONE_ID: required with ${E}_CLOUDFLARE_TOKEN")
    fi
  done

  # TLS: Cloudflare Origin CA certificate and key, unless already installed.
  V_TLS_CERT=""; V_TLS_KEY=""; V_TLS_NEW=0
  if [ -f "$ROOT/certs/origin.pem" ] && [ -f "$ROOT/certs/origin.key" ] && [ "$FORCE" != 1 ]; then
    if [ -n "${AWT_TLS_CERT_FILE:-}${AWT_TLS_KEY_FILE:-}" ]; then
      warn "AWT_TLS_CERT_FILE/AWT_TLS_KEY_FILE: keeping the certificate already installed in $ROOT/certs (AWT_INIT_FORCE=1 replaces it)"
    fi
  elif [ -n "${AWT_TLS_CERT_FILE:-}" ] || [ -n "${AWT_TLS_KEY_FILE:-}" ] || [ "$INTERACTIVE" = 0 ]; then
    local msg
    for f in AWT_TLS_CERT_FILE AWT_TLS_KEY_FILE; do
      if [ -z "${!f:-}" ]; then missing+=("$f")
      elif ! msg=$(check_value pemfile "${!f}"); then errors+=("$f: $msg")
      fi
    done
    if [ -n "${AWT_TLS_CERT_FILE:-}" ] && [ -n "${AWT_TLS_KEY_FILE:-}" ] && [ -r "$AWT_TLS_CERT_FILE" ] && [ -r "$AWT_TLS_KEY_FILE" ]; then
      V_TLS_CERT=$(cat "$AWT_TLS_CERT_FILE"); V_TLS_KEY=$(cat "$AWT_TLS_KEY_FILE"); V_TLS_NEW=1
    fi
  else
    V_TLS_CERT=$(read_pem "Cloudflare Origin CA certificate" 0)
    V_TLS_KEY=$(read_pem "its private key" 1)
    V_TLS_NEW=1
  fi
  [ "$V_TLS_NEW" = 0 ] || new_inputs=1

  if [ -n "$V_AWT_PROD_HOST" ] && [ "$V_AWT_PROD_HOST" = "$V_AWT_TRAINING_HOST" ]; then
    errors+=("AWT_TRAINING_HOST: must differ from AWT_PROD_HOST")
  fi
  if [ -n "$V_PROD_MFA_KEY" ] && [ "$V_PROD_MFA_KEY" = "$V_TRAINING_MFA_KEY" ]; then
    errors+=("TRAINING_MFA_KEY: must differ from PROD_MFA_KEY")
  fi

  if [ "${#missing[@]}" -gt 0 ] || [ "${#errors[@]}" -gt 0 ]; then
    {
      echo "ERROR: cannot continue, nothing was changed."
      if [ "${#missing[@]}" -gt 0 ]; then
        echo "  Missing values (stdin is not a terminal: set them in the environment):"
        for f in "${missing[@]}"; do printf '    %-28s %s\n' "$f" "$(desc_of "$f")"; done
      fi
      if [ "${#errors[@]}" -gt 0 ]; then
        echo "  Invalid values:"
        for f in "${errors[@]}"; do echo "    $f"; done
      fi
      echo "  See docs/ops.md section 0 for every variable."
    } >&2
    exit 1
  fi
  [ "$new_inputs" = 1 ] && step_changed=1
  return 0
}

step_packages() {
  local p need=()
  for p in ca-certificates curl gnupg openssl sudo nftables openssh-server fail2ban python3-systemd unattended-upgrades; do
    dpkg-query -W -f='${db:Status-Abbrev}' "$p" 2>/dev/null | grep -q '^ii' || need+=("$p")
  done
  if [ "${#need[@]}" -gt 0 ]; then
    note "installing ${need[*]}"
    apt-get update -q >/dev/null
    DEBIAN_FRONTEND=noninteractive apt-get install -y -q --no-install-recommends "${need[@]}" >/dev/null
    changed "installed ${need[*]}"
  fi
  # What `dpkg-reconfigure -plow unattended-upgrades` writes (docs/ops.md 1.6).
  put_file /etc/apt/apt.conf.d/20auto-upgrades root:root 644 <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
}

step_users_ssh() {
  local user=$V_AWT_ADMIN_USER port=$V_AWT_SSH_PORT home grp ak keys tmp
  if ! id -u "$user" >/dev/null 2>&1; then
    useradd -m -s /bin/bash -G sudo "$user"
    changed "created admin user $user"
  elif ! id -nG "$user" | grep -qw sudo; then
    usermod -aG sudo "$user"
    changed "added $user to sudo"
  fi
  home=$(getent passwd "$user" | cut -d: -f6)
  grp=$(id -gn "$user")
  ensure_dir "$home/.ssh" "$user:$grp" 700
  ak=$home/.ssh/authorized_keys
  keys=$(cat "$ak" 2>/dev/null || true)
  if [ -n "$V_AWT_ADMIN_SSH_KEY" ]; then
    local type blob rest
    read -r type blob rest <<<"$V_AWT_ADMIN_SSH_KEY"
    if ! printf '%s\n' "$keys" | awk -v t="$type" -v b="$blob" '$1 == t && $2 == b { f = 1 } END { exit !f }'; then
      keys=$(printf '%s\n%s' "$keys" "$V_AWT_ADMIN_SSH_KEY" | sed '/^$/d')
    fi
  fi
  put_file "$ak" "$user:$grp" 600 <<<"$keys"
  # Never restrict sshd unless the admin can log in with a key.
  ssh-keygen -l -f "$ak" >/dev/null 2>&1 \
    || die "no valid SSH public key in $ak: refusing to reload sshd (you would be locked out); the SSH configuration is unchanged"

  # Key-only login, so the admin needs sudo without a password.
  tmp=$(mktemp)
  printf '%s ALL=(ALL) NOPASSWD:ALL\n' "$user" > "$tmp"
  visudo -cqf "$tmp" || { rm -f "$tmp"; die "sudoers line for $user rejected by visudo"; }
  put_file "$SUDOERS" root:root 440 < "$tmp"
  rm -f "$tmp"

  # sshd: first value wins, so this file (10-) overrides later ones such as
  # 50-cloud-init.conf. Tested with `sshd -t` and `sshd -T` before any reload.
  local old="" had=0 content eff ok=1 want
  content=$(printf '# Managed by deploy/init-vps.sh (docs/ops.md section 1).\nPort %s\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin no\nAllowUsers %s\n' "$port" "$user")
  if [ "$(cat "$SSHD_CONF" 2>/dev/null || true)" != "$content" ]; then
    if [ -f "$SSHD_CONF" ]; then had=1; old=$(cat "$SSHD_CONF"); fi
    put_file "$SSHD_CONF" root:root 644 <<<"$content"
    restore_sshd() {
      if [ "$had" = 1 ]; then printf '%s\n' "$old" > "$SSHD_CONF"; else rm -f "$SSHD_CONF"; fi
    }
    if ! eff=$(sshd -t 2>&1); then
      restore_sshd
      die "sshd -t rejects the SSH configuration: $eff
       refusing to reload sshd; the previous SSH configuration is left in place"
    fi
    eff=$(sshd -T 2>/dev/null || true)
    for want in "port $port" "passwordauthentication no" "permitrootlogin no" "allowusers $user"; do
      printf '%s\n' "$eff" | grep -qx "$want" || { ok=0; warn "sshd would not apply '$want' (another file in /etc/ssh overrides it)"; }
    done
    if [ "$ok" = 0 ]; then
      restore_sshd
      die "refusing to reload sshd; the previous SSH configuration is left in place"
    fi
    # Open the new port before sshd moves to it (the firewall may already be up).
    if nft list table inet awt >/dev/null 2>&1; then
      nft insert rule inet awt input tcp dport "$port" accept
    fi
    # Socket activation ignores Port in sshd_config: run the plain service.
    if systemctl is-enabled ssh.socket >/dev/null 2>&1 || systemctl is-active ssh.socket >/dev/null 2>&1; then
      systemctl disable --now ssh.socket
      systemctl enable --now ssh.service
      changed "ssh.socket replaced by ssh.service"
    fi
    systemctl reload ssh.service
    changed "sshd reloaded: port $port, key-only, AllowUsers $user"
    warn "SSH now listens on port $port for $user only. Keep this session open and test a new SSH session (ssh -p $port $user@<this host>) before closing this one."
  fi

  # fail2ban: Debian 13 logs to the journal only.
  put_file "$F2B_JAIL" root:root 644 <<EOF
# Managed by deploy/init-vps.sh (docs/ops.md section 1).
[sshd]
enabled = true
port = $port
backend = systemd
banaction = nftables[type=multiport]
banaction_allports = nftables[type=allports]
EOF
  if [ "$step_changed" = 1 ] || ! systemctl is-enabled fail2ban >/dev/null 2>&1; then
    systemctl enable fail2ban >/dev/null 2>&1
    systemctl restart fail2ban
  fi
}

# cf_ranges <4|6>: Cloudflare ranges, fetched (validated) or the pinned list.
cf_ranges() {
  local list re
  if [ "$1" = 4 ]; then re='^[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,2}$'; else re='^[0-9a-f:]+/[0-9]{1,3}$'; fi
  list=$(curl -fsS --max-time 15 "https://www.cloudflare.com/ips-v$1" 2>/dev/null | tr -d '\r' || true)
  if [ -n "$list" ] && ! printf '%s\n' "$list" | grep -Evq "$re"; then
    printf '%s\n' "$list" | paste -sd ' ' -
  else
    warn "cannot fetch https://www.cloudflare.com/ips-v$1: using the list pinned in this script"
    if [ "$1" = 4 ]; then echo "$CF4_FALLBACK"; else echo "$CF6_FALLBACK"; fi
  fi
}

step_firewall() {
  local port=$V_AWT_SSH_PORT cf4 cf6 tmp
  cf4=$(cf_ranges 4 | sed 's/ /, /g')
  cf6=$(cf_ranges 6 | sed 's/ /, /g')
  tmp=$(mktemp)
  # Docker publishes Caddy's ports with DNAT in prerouting: that traffic goes
  # through the forward hook, never the input chain. A drop in any base chain
  # is final in nftables, so the forward chain below blocks it whatever
  # Docker's own (iptables-nft) chains accept, and survives Docker restarts
  # (Docker never touches this table). Connections Docker's userland proxy
  # takes (IPv6 to the host) do reach the input chain. Both chains allow
  # 80/443 from Cloudflare only.
  cat > "$tmp" <<EOF
# Managed by deploy/init-vps.sh (docs/ops.md section 1). Loaded by
# /etc/nftables.conf; reload with: nft -f /etc/nftables.conf
table inet awt {}
delete table inet awt
table inet awt {
	set cloudflare4 {
		type ipv4_addr; flags interval
		elements = { $cf4 }
	}
	set cloudflare6 {
		type ipv6_addr; flags interval
		elements = { $cf6 }
	}
	chain input {
		type filter hook input priority filter; policy drop;
		ct state established,related accept
		ct state invalid drop
		iif "lo" accept
		meta l4proto { icmp, ipv6-icmp } accept
		udp sport 67 udp dport 68 accept
		udp sport 547 udp dport 546 accept
		tcp dport $port accept
		ip saddr @cloudflare4 tcp dport { 80, 443 } accept
		ip6 saddr @cloudflare6 tcp dport { 80, 443 } accept
		ip saddr @cloudflare4 udp dport 443 accept
		ip6 saddr @cloudflare6 udp dport 443 accept
	}
	chain forward {
		type filter hook forward priority filter - 10; policy accept;
		ct state new ct status dnat meta l4proto tcp ct original proto-dst { 80, 443 } ip saddr != @cloudflare4 drop
		ct state new ct status dnat meta l4proto tcp ct original proto-dst { 80, 443 } ip6 saddr != @cloudflare6 drop
		ct state new ct status dnat meta l4proto udp ct original proto-dst 443 ip saddr != @cloudflare4 drop
		ct state new ct status dnat meta l4proto udp ct original proto-dst 443 ip6 saddr != @cloudflare6 drop
	}
}
EOF
  nft -c -f "$tmp" || { rm -f "$tmp"; die "nft rejects the generated ruleset (nothing loaded)"; }
  ensure_dir /etc/nftables.d root:root 755
  put_file "$NFT_FILE" root:root 644 < "$tmp"
  rm -f "$tmp"
  # Debian's default nftables.conf starts with "flush ruleset", which would
  # also delete Docker's tables at every reload: keep a copy, include ours.
  if [ -f /etc/nftables.conf ] && ! grep -q 'deploy/init-vps.sh' /etc/nftables.conf && [ ! -f /etc/nftables.conf.dist ]; then
    cp -p /etc/nftables.conf /etc/nftables.conf.dist
    changed "saved the previous /etc/nftables.conf as /etc/nftables.conf.dist"
  fi
  put_file /etc/nftables.conf root:root 755 <<'EOF'
#!/usr/sbin/nft -f
# Managed by deploy/init-vps.sh (docs/ops.md section 1). No "flush ruleset":
# each file replaces only its own table, so Docker's tables survive a reload.
include "/etc/nftables.d/*.nft"
EOF
  # Stopping nftables.service would run "nft flush ruleset" (Docker's rules too).
  ensure_dir /etc/systemd/system/nftables.service.d root:root 755
  local before=$step_changed
  step_changed=0
  put_file /etc/systemd/system/nftables.service.d/awt.conf root:root 644 <<'EOF'
# Managed by deploy/init-vps.sh: remove only our table, never Docker's.
[Service]
ExecStop=
ExecStop=-/usr/sbin/nft delete table inet awt
EOF
  [ "$step_changed" = 0 ] || systemctl daemon-reload
  [ "$before" = 0 ] || step_changed=1
  if [ "$step_changed" = 1 ] || ! nft list table inet awt >/dev/null 2>&1; then
    nft -c -f /etc/nftables.conf
    nft -f /etc/nftables.conf
    changed "firewall loaded: SSH $port from anywhere, 80/443 from Cloudflare only"
  fi
  if ! systemctl is-enabled nftables >/dev/null 2>&1; then
    systemctl enable nftables
    changed "nftables enabled at boot"
  fi
}

compose_ok() {
  local v
  v=$(docker compose version --short 2>/dev/null) || return 1
  v=${v#v}
  [[ $v =~ ^([0-9]+)\.([0-9]+) ]] || return 1
  [ "${BASH_REMATCH[1]}" -gt 2 ] || { [ "${BASH_REMATCH[1]}" = 2 ] && [ "${BASH_REMATCH[2]}" -ge 22 ]; }
}

step_docker() {
  if ! compose_ok; then
    local tmp gh fpr codename arch
    tmp=$(mktemp)
    curl -fsSL --max-time 60 -o "$tmp" https://download.docker.com/linux/debian/gpg \
      || die "cannot download Docker's apt key"
    gh=$(mktemp -d)
    fpr=$(GNUPGHOME=$gh gpg --show-keys --with-colons "$tmp" 2>/dev/null | awk -F: '$1 == "fpr" { print $10; exit }')
    rm -rf "$gh"
    [ "$fpr" = "$DOCKER_KEY_FPR" ] || { rm -f "$tmp"; die "Docker's apt key has fingerprint '$fpr', expected $DOCKER_KEY_FPR"; }
    ensure_dir /etc/apt/keyrings root:root 755
    put_file "$DOCKER_KEY" root:root 644 < "$tmp"
    rm -f "$tmp"
    codename=$(. /etc/os-release && echo "$VERSION_CODENAME")
    arch=$(dpkg --print-architecture)
    put_file /etc/apt/sources.list.d/docker.sources root:root 644 <<EOF
Types: deb
URIs: https://download.docker.com/linux/debian
Suites: $codename
Components: stable
Architectures: $arch
Signed-By: $DOCKER_KEY
EOF
    note "installing Docker Engine and the compose plugin"
    apt-get update -q >/dev/null
    DEBIAN_FRONTEND=noninteractive apt-get install -y -q docker-ce docker-ce-cli containerd.io \
      docker-buildx-plugin docker-compose-plugin >/dev/null
    changed "installed Docker Engine"
    compose_ok || die "Docker Compose 2.22 or later is required (docker compose version)"
  fi
  # Log rotation (docs/ops.md section 7). An existing file is left alone.
  if [ ! -f /etc/docker/daemon.json ]; then
    ensure_dir /etc/docker root:root 755
    put_file /etc/docker/daemon.json root:root 644 <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "5" }
}
EOF
    systemctl restart docker
  fi
  if ! systemctl is-enabled docker >/dev/null 2>&1 || ! systemctl is-active docker >/dev/null 2>&1; then
    systemctl enable --now docker
    changed "docker enabled and started"
  fi
}

step_layout() {
  if ! id -u deploy >/dev/null 2>&1; then
    useradd -m -s /usr/sbin/nologin -G docker deploy
    changed "created user deploy (docker group, no login shell)"
  elif ! id -nG deploy | grep -qw docker; then
    usermod -aG docker deploy
    changed "added deploy to docker"
  fi
  # deploy has no SSH access (docs/ops.md 1.4).
  local dh f tmp
  dh=$(getent passwd deploy | cut -d: -f6)
  if [ -e "$dh/.ssh/authorized_keys" ]; then
    rm -f "$dh/.ssh/authorized_keys"
    changed "removed $dh/.ssh/authorized_keys (deploy has no SSH access)"
  fi
  ensure_dir "$ROOT" deploy:deploy 750
  ensure_dir "$ROOT/state" deploy:deploy 750
  ensure_dir "$ROOT/caddy" deploy:deploy 755
  ensure_dir "$ROOT/certs" deploy:deploy 700
  ensure_dir "$ROOT/deploy" root:root 755
  tmp=$(mktemp)
  for f in agent.sh deploy.sh weekly-restore-test.sh; do
    fetch "deploy/$f" "$tmp"
    head -1 "$tmp" | grep -q '^#!' || die "deploy/$f from $V_AWT_REPO_RAW is not a script"
    put_file "$ROOT/deploy/$f" root:root 755 < "$tmp"
  done
  rm -f "$tmp"
}

step_tls() {
  local cert=$ROOT/certs/origin.pem key=$ROOT/certs/origin.key c k h
  if [ "$V_TLS_NEW" = 0 ]; then return 0; fi
  c=$(mktemp); k=$(mktemp)
  printf '%s\n' "$V_TLS_CERT" > "$c"
  printf '%s\n' "$V_TLS_KEY" > "$k"
  tls_fail() { rm -f "$c" "$k"; die "$1"; }
  openssl x509 -in "$c" -noout 2>/dev/null || tls_fail "the TLS certificate is not a PEM certificate"
  openssl pkey -in "$k" -noout 2>/dev/null || tls_fail "the TLS key is not a PEM private key"
  [ "$(openssl x509 -in "$c" -noout -pubkey)" = "$(openssl pkey -in "$k" -pubout)" ] \
    || tls_fail "the TLS certificate and key do not match"
  openssl x509 -in "$c" -noout -checkend 0 >/dev/null || tls_fail "the TLS certificate has expired"
  for h in "$V_AWT_PROD_HOST" "$V_AWT_TRAINING_HOST"; do
    openssl x509 -in "$c" -noout -checkhost "$h" | grep -q ' does match' \
      || tls_fail "the TLS certificate does not cover $h"
  done
  put_file "$cert" deploy:deploy 644 < "$c"
  put_file "$key" deploy:deploy 600 < "$k"
  rm -f "$c" "$k"
  if [ "$step_changed" = 1 ] && [ -f "$ROOT/compose.yaml" ]; then
    note "certificate replaced: restart Caddy with: cd $ROOT && docker compose restart caddy"
  fi
}

# replica_exists <env>: whether the env's bucket holds a Litestream replica,
# asked to litestream itself in a one-shot container of the app image (same
# litestream.yml as the app). litestream exits 0 on S3 errors, so they are
# detected in its log.
replica_exists() {
  local e=$1 E=${1^^} image="$V_AWT_IMAGE:$V_AWT_CHANNEL" out err rc=0 v
  local bucket="V_${E}_S3_BUCKET"
  docker pull -q "$image" >/dev/null 2>&1 \
    || die "cannot pull $image to check s3://${!bucket}/litestream/$e for a replica (is the image published?); or set ${E}_MFA_KEY"
  out=$(mktemp); err=$(mktemp)
  for v in S3_ENDPOINT S3_BUCKET S3_ACCESS_KEY S3_SECRET_KEY; do
    local src="V_${E}_$v"; export "AWT_$v=${!src}"
  done
  local region="V_${E}_S3_REGION"; export LITESTREAM_S3_REGION="${!region}"
  # Secrets go by name (-e NAME), never on the command line.
  docker run --rm --entrypoint litestream -e "AWT_ENV=$e" -e AWT_S3_ENDPOINT -e AWT_S3_BUCKET \
    -e AWT_S3_ACCESS_KEY -e AWT_S3_SECRET_KEY -e LITESTREAM_S3_REGION \
    "$image" snapshots -config /etc/litestream.yml /data/awt.db >"$out" 2>"$err" || rc=$?
  unset AWT_S3_ENDPOINT AWT_S3_BUCKET AWT_S3_ACCESS_KEY AWT_S3_SECRET_KEY LITESTREAM_S3_REGION
  if [ "$rc" != 0 ] || grep -q 'level=ERROR' "$err"; then
    local why
    why=$(grep -m1 'level=ERROR' "$err" || tail -1 "$err")
    rm -f "$out" "$err"
    die "cannot list s3://${!bucket}/litestream/$e: $why
       Fix ${E}_S3_* (stored values are kept unless AWT_INIT_FORCE=1), or set ${E}_MFA_KEY."
  fi
  rc=$(tail -n +2 "$out" | grep -c . || true)
  rm -f "$out" "$err"
  [ "$rc" -gt 0 ]
}

# write_env <file> <example> KEY=VALUE...: a new file starts from the example
# with the given values; an existing one only gets missing keys (from the
# example) and values for keys that are missing or empty, or that changed
# with AWT_INIT_FORCE=1 (resolve() keeps stored values otherwise). The image
# tags (AWT_*_TAG) belong to deploy.sh and are never changed here.
write_env() {
  local dest=$1 example=$2 tmp kv k v cur
  shift 2
  tmp=$(mktemp)
  if [ -f "$dest" ]; then
    cp "$dest" "$tmp"
    while IFS= read -r line; do
      [[ $line =~ ^([A-Z_][A-Z0-9_]*)= ]] || continue
      k=${BASH_REMATCH[1]}
      grep -q "^$k=" "$tmp" || printf '%s\n' "$line" >> "$tmp"
    done < "$example"
  else
    cp "$example" "$tmp"
  fi
  for kv in "$@"; do
    k=${kv%%=*}; v=${kv#*=}
    [ -n "$v" ] || continue
    cur=$(file_value "$tmp" "$k")
    if ! grep -q "^$k=" "$tmp" || [ "$cur" != "$v" ]; then set_key "$tmp" "$k" "$v"; fi
  done
  put_file "$dest" deploy:deploy 600 < "$tmp"
  rm -f "$tmp"
}

step_env_files() {
  local e E ex
  # MFA key: kept when stored, used when given; otherwise generated, unless
  # the bucket already holds a database: its TOTP secrets are encrypted with
  # the existing key, and a new one would make every enrolled app unusable.
  for e in "${ENVS[@]}"; do
    E=${e^^}
    local name="${E}_MFA_KEY" var="V_${E}_MFA_KEY" bucket="V_${E}_S3_BUCKET"
    [ -z "${!var}" ] || continue
    if replica_exists "$e"; then
      local why="s3://${!bucket}/litestream/$e holds a Litestream replica: the $e database restored from it may hold TOTP secrets encrypted with its existing MFA key. A new key would make every enrolled authenticator app unusable (docs/ops.md section 7b), so none is generated."
      if [ "$INTERACTIVE" = 1 ]; then
        warn "$why Enter the existing $name (from your secrets backup)."
        ask "$name" 1 0
      else
        die "$why Set $name to the existing key (from your secrets backup) and re-run."
      fi
    else
      printf -v "$var" '%s' "$(openssl rand -base64 32)"
      note "generated $name (no replica in s3://${!bucket}/litestream/$e). Back it up with the other secrets."
    fi
    if [ "$e" = training ] && [ "$V_TRAINING_MFA_KEY" = "${V_PROD_MFA_KEY:-}" ]; then
      die "TRAINING_MFA_KEY must differ from PROD_MFA_KEY"
    fi
  done

  ex=$(mktemp -d)
  fetch deploy/env.example "$ex/env"
  fetch deploy/env.prod.example "$ex/prod"
  fetch deploy/env.training.example "$ex/training"
  grep -q '^AWT_' "$ex/env" || die "deploy/env.example from $V_AWT_REPO_RAW has no AWT_ keys"
  write_env "$ROOT/.env" "$ex/env" "AWT_IMAGE=$V_AWT_IMAGE" "AWT_CHANNEL=$V_AWT_CHANNEL" \
    "AWT_PROD_HOST=$V_AWT_PROD_HOST" "AWT_TRAINING_HOST=$V_AWT_TRAINING_HOST"
  for e in "${ENVS[@]}"; do
    E=${e^^}
    local h="V_AWT_${E}_HOST" s1="V_${E}_S3_ENDPOINT" s2="V_${E}_S3_REGION" s3="V_${E}_S3_BUCKET" \
      s4="V_${E}_S3_ACCESS_KEY" s5="V_${E}_S3_SECRET_KEY" m="V_${E}_MFA_KEY" \
      z="V_${E}_CLOUDFLARE_ZONE_ID" t="V_${E}_CLOUDFLARE_TOKEN"
    write_env "$ROOT/.env.$e" "$ex/$e" "AWT_BASE_URL=https://${!h}" "AWT_S3_ENDPOINT=${!s1}" \
      "LITESTREAM_S3_REGION=${!s2}" "AWT_S3_BUCKET=${!s3}" "AWT_S3_ACCESS_KEY=${!s4}" \
      "AWT_S3_SECRET_KEY=${!s5}" "AWT_MFA_KEY=${!m}" "AWT_CLOUDFLARE_ZONE_ID=${!z}" \
      "AWT_CLOUDFLARE_TOKEN=${!t}"
  done
  rm -rf "$ex"
}

step_agent() {
  local u tmp before=$step_changed
  tmp=$(mktemp)
  for u in awt-agent.service awt-agent.timer; do
    fetch "deploy/$u" "$tmp"
    grep -q '^\[Unit\]' "$tmp" || die "deploy/$u from $V_AWT_REPO_RAW is not a systemd unit"
    put_file "/etc/systemd/system/$u" root:root 644 < "$tmp"
  done
  rm -f "$tmp"
  [ "$step_changed" = "$before" ] || systemctl daemon-reload
  if ! systemctl is-enabled awt-agent.timer >/dev/null 2>&1 || ! systemctl is-active awt-agent.timer >/dev/null 2>&1; then
    systemctl enable --now awt-agent.timer
    changed "awt-agent.timer enabled (the agent checks for a new image every 60 s)"
  fi
}

deploy_failed() {
  {
    echo "ERROR: $1"
    echo "---- journalctl -u awt-agent (last 50 lines) ----"
    journalctl -u awt-agent.service -n 50 --no-pager 2>&1 || true
    echo "----"
    echo "Next steps (everything else is installed; the timer keeps trying every 60 s):"
    echo "  - read the agent log above; follow it with: journalctl -u awt-agent -f"
    echo "  - image not published yet: push to the deploy branch ($V_AWT_CHANNEL) and wait for CI"
    echo "  - app not healthy: cd $ROOT && docker compose logs awt-training awt-prod caddy"
    echo "    (S3 settings in .env.training / .env.prod, certificate in certs/)"
    echo "  - a revision recorded in $ROOT/state/failed-revisions is skipped: delete its line to retry"
    echo "  - then re-run this script: it resumes at the first deploy (docs/ops.md sections 3 and 5)"
  } >&2
  exit 1
}

healthy() {
  dc exec -T "awt-$1" wget -q -O /dev/null http://127.0.0.1:8080/healthz >/dev/null 2>&1 \
    && dc exec -T caddy wget -q -O /dev/null "http://127.0.0.1:8081/$1/healthz" >/dev/null 2>&1
}

step_first_deploy() {
  local t p e
  t=$(file_value "$ROOT/.env" AWT_TRAINING_TAG); p=$(file_value "$ROOT/.env" AWT_PROD_TAG)
  if [ "${t:-none}" = none ] || [ "${p:-none}" = none ]; then
    note "first deploy: running the deploy agent once (follow it with: journalctl -u awt-agent -f)"
    systemctl start awt-agent.service || deploy_failed "the deploy agent failed"
    t=$(file_value "$ROOT/.env" AWT_TRAINING_TAG); p=$(file_value "$ROOT/.env" AWT_PROD_TAG)
    if [ "${t:-none}" = none ] || [ "${p:-none}" = none ]; then
      deploy_failed "the deploy agent finished but deployed nothing (training=${t:-none} prod=${p:-none})"
    fi
    changed "deployed training=$t prod=$p"
  fi
  for e in "${ENVS[@]}"; do
    healthy "$e" || deploy_failed "awt-$e does not answer /healthz (directly or through Caddy)"
  done
}

# db_status <env>: how the database started (Litestream's log in the
# container) and whether it holds any user.
db_status() {
  local logs users
  logs=$(dc logs --no-log-prefix "awt-$1" 2>/dev/null || true)
  users=$(dc exec -T "awt-$1" sqlite3 /data/awt.db 'SELECT count(*) FROM users' 2>/dev/null | tr -d '\r' || true)
  if printf '%s' "$logs" | grep -q 'msg="restoring snapshot"'; then
    echo "restored from the S3 replica (${users:-?} users)"
  elif [ "$users" = 0 ]; then
    echo "empty"
  elif [[ $users =~ ^[0-9]+$ ]]; then
    echo "existing database ($users users)"
  else
    echo "unknown (cd $ROOT && docker compose logs awt-$1)"
  fi
}

step_summary() {
  local e E s empty=()
  log ""
  log "AcroWorldTour host ready (docs/ops.md)."
  log "  admin SSH:  ssh -p $V_AWT_SSH_PORT $V_AWT_ADMIN_USER@<this host>   (password login and root login disabled)"
  log "  sites:      https://$V_AWT_PROD_HOST  https://$V_AWT_TRAINING_HOST"
  log "  image:      $V_AWT_IMAGE:$V_AWT_CHANNEL, training=$(file_value "$ROOT/.env" AWT_TRAINING_TAG) prod=$(file_value "$ROOT/.env" AWT_PROD_TAG)"
  for e in "${ENVS[@]}"; do
    E=${e^^}
    local b="V_${E}_S3_BUCKET" a="V_${E}_S3_ACCESS_KEY" sk="V_${E}_S3_SECRET_KEY" m="V_${E}_MFA_KEY" t="V_${E}_CLOUDFLARE_TOKEN"
    s=$(db_status "$e")
    log "  $e: bucket ${!b}, S3 access key $(mask "${!a}"), S3 secret $(mask "${!sk}"), MFA key $(mask "${!m}"), Cloudflare token $(mask "${!t}")"
    log "  $e database: $s"
    [ "$s" != empty ] || empty+=("$e")
  done
  if [ "${#empty[@]}" -gt 0 ]; then
    log ""
    log "Empty database for: ${empty[*]}. Import the legacy data, then create the first admin (docs/ops.md sections 3 and 4):"
    log "  scp -P $V_AWT_SSH_PORT mongo.backup.YYYYMMDD-HHMM.tar.gz $V_AWT_ADMIN_USER@<this host>:/tmp/"
    log "  cd $ROOT"
    for e in "${empty[@]}"; do
      log "  docker compose cp /tmp/mongo.backup.YYYYMMDD-HHMM.tar.gz awt-$e:/tmp/export.tar.gz"
      log "  docker compose exec awt-$e awt import --from /tmp/export.tar.gz"
      log "  docker compose exec awt-$e awt user add --email you@example.org --name \"You\" --role admin"
    done
    log "  (training first, check it in the browser, then prod; delete the archive afterwards)"
  fi
  log ""
  log "Left to do by hand (docs/ops.md section 0):"
  log "  - test a new SSH session on port $V_AWT_SSH_PORT before closing this one"
  log "  - Cloudflare: proxied DNS for both hosts, SSL mode Full (strict), cache bypass for /live/*"
  log "  - back up $ROOT/.env.prod and $ROOT/.env.training (MFA keys, S3 secrets) with your other secrets"
  log "  - weekly restore test cron (section 6) and an uptime check on https://$V_AWT_PROD_HOST/healthz (section 7)"
}

main() {
  current_step=preflight; step_preflight; log "==> preflight: done"
  current_step=inputs; step_inputs
  if [ "$step_changed" = 1 ]; then log "==> inputs: done"; else log "==> inputs: already done"; fi
  run_step packages
  run_step users_ssh
  run_step firewall
  run_step docker
  run_step layout
  run_step tls
  run_step env_files
  run_step agent
  run_step first_deploy
  current_step=summary; step_summary; log "==> summary: done"
  current_step=""
}

main "$@"
