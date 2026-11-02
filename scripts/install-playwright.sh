#!/usr/bin/env bash
# Installs what `make e2e` needs: the Playwright driver and chromium.
#
# playwright-go v0.6201.1 (now github.com/mxschmitt/playwright-go) assembles
# its driver from the playwright-core npm
# package and a Node.js binary, but checks neither download. They are
# therefore fetched here at pinned versions, checked against their sha256
# (npm's published sha512 integrity and nodejs.org's SHASUMS256.txt give the
# same files) and laid out where playwright-go looks for them; the pinned
# playwright-go CLI then finds an up-to-date driver and installs chromium.
# Idempotent. No sudo, no system packages. linux-x64 only, like CI.
set -euo pipefail

PW_GO_VERSION=v0.6201.1
DRIVER_VERSION=1.62.1 # playwrightCliVersion of PW_GO_VERSION
CORE_URL=https://registry.npmjs.org/playwright-core/-/playwright-core-$DRIVER_VERSION.tgz
CORE_SHA256=954be1e183d0ddb9748fe0d2d08b0b66a9210c74dd75c397aeb70303b9f08a00
NODE_VERSION=24.19.0 # nodeVersion of PW_GO_VERSION
NODE_DIR=node-v$NODE_VERSION-linux-x64
NODE_URL=https://nodejs.org/dist/v$NODE_VERSION/$NODE_DIR.tar.gz
NODE_SHA256=f625d97cd707df4ff96254916fbc5ff014f09c09effe5a1e0ca8f6d41a8789d4

dest="${XDG_CACHE_HOME:-$HOME/.cache}/ms-playwright-go/$DRIVER_VERSION"

if [ ! -x "$dest/node" ] || [ ! -f "$dest/package/cli.js" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	curl -fsSL -o "$tmp/core.tgz" "$CORE_URL"
	echo "$CORE_SHA256  $tmp/core.tgz" | sha256sum -c -
	curl -fsSL -o "$tmp/node.tar.gz" "$NODE_URL"
	echo "$NODE_SHA256  $tmp/node.tar.gz" | sha256sum -c -
	rm -rf "$dest"
	mkdir -p "$dest"
	tar -xzf "$tmp/core.tgz" -C "$dest" package
	tar -xzf "$tmp/node.tar.gz" -C "$tmp" "$NODE_DIR/bin/node"
	cp "$tmp/$NODE_DIR/bin/node" "$dest/node"
	chmod +x "$dest/node"
fi

cd "$(dirname "$0")/.."
go run "github.com/mxschmitt/playwright-go/cmd/playwright@$PW_GO_VERSION" install chromium
