.PHONY: dev generate test golden golden-regen e2e lint check migrate docker backup-restore-test e2e-deps

GO ?= go

dev:
	AWT_ENV=dev $(GO) run ./cmd/awt serve

generate:
	@if [ -n "$$(find . -name '*.templ' -not -path './legacy/*' | head -1)" ]; then $(GO) tool templ generate; else echo "no .templ files, skipping templ"; fi
	@if [ -f sqlc.yaml ]; then $(GO) tool sqlc generate; else echo "no sqlc.yaml, skipping sqlc"; fi

test:
	$(GO) test -race ./...

golden:
	$(GO) test ./internal/scoring ./internal/ranking -run Golden -count=1

# Regenerate testdata/golden from a production export with the legacy engine
# (see tools/golden/README.md). Runs from a temporary directory with -I so
# that nothing in the repository or the export is imported as code.
golden-regen:
	@test -n "$(LEGACY_VENV)" -a -n "$(BACKUP)" || { echo "usage: make golden-regen LEGACY_VENV=<venv dir> BACKUP=<export.tar.gz>"; exit 1; }
	d="$$(mktemp -d)" && trap 'rm -rf "$$d"' EXIT && cd "$$d" && \
		"$(abspath $(LEGACY_VENV))/bin/python" -I "$(CURDIR)/tools/golden/generate.py" \
		--backup "$(abspath $(BACKUP))" --legacy "$(CURDIR)/legacy/legacy" --out "$(CURDIR)/testdata/golden"

# Driver and chromium for e2e (once; see scripts/install-playwright.sh).
e2e-deps:
	./scripts/install-playwright.sh

e2e:
	AWT_E2E=1 $(GO) test ./e2e/... -count=1

lint:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; echo "gofmt: files need formatting"; exit 1; }
	$(GO) vet ./...
	$(GO) tool staticcheck ./...
	$(GO) tool govulncheck ./...
	! grep -rn 'math.Round(' --include=*.go internal/ cmd/

check: generate
	@test -z "$$(git status --porcelain)" || { git status --porcelain; echo "generate left a diff: commit generated files"; exit 1; }
	$(MAKE) lint test golden e2e

migrate:
	$(GO) run ./cmd/awt migrate

docker:
	docker build -t awt:dev .

# Restore test: MinIO + litestream replicate, write data, wipe the volume,
# restart, check the data came back. See scripts/backup-restore-test.sh.
backup-restore-test: docker
	./scripts/backup-restore-test.sh
