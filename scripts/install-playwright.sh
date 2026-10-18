#!/usr/bin/env bash
# Installs what `make e2e` needs: the Playwright driver and chromium.
#
# playwright-go v0.5700.1 downloads its driver from playwright.azureedge.net,
# which no longer serves it (404). The driver (node + package) is therefore
# taken from the pinned PyPI wheel below, checked against its sha256 and
# unpacked where playwright-go looks for it, then the pinned playwright-go CLI
# installs chromium. Idempotent. No sudo, no system packages.
set -euo pipefail

PW_GO_VERSION=v0.5700.1
DRIVER_VERSION=1.57.0
WHEEL=playwright-1.57.0-py3-none-manylinux1_x86_64.whl
WHEEL_URL=https://files.pythonhosted.org/packages/56/61/3a803cb5ae0321715bfd5247ea871d25b32c8f372aeb70550a90c5f586df/$WHEEL
WHEEL_SHA256=284ed5a706b7c389a06caa431b2f0ba9ac4130113c3a779767dda758c2497bb1

dest="${XDG_CACHE_HOME:-$HOME/.cache}/ms-playwright-go/$DRIVER_VERSION"

if [ ! -x "$dest/node" ] || [ ! -f "$dest/package/cli.js" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	curl -fsSL -o "$tmp/$WHEEL" "$WHEEL_URL"
	echo "$WHEEL_SHA256  $tmp/$WHEEL" | sha256sum -c -
	# -I: isolated mode, nothing from the archive or the cwd is imported.
	python3 -I -c 'import sys, zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' "$tmp/$WHEEL" "$tmp/x"
	rm -rf "$dest"
	mkdir -p "$dest"
	cp -r "$tmp/x/playwright/driver/package" "$dest/package"
	cp "$tmp/x/playwright/driver/node" "$dest/node"
	chmod +x "$dest/node"
fi

cd "$(dirname "$0")/.."
go run "github.com/playwright-community/playwright-go/cmd/playwright@$PW_GO_VERSION" install chromium
