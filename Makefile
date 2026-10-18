.PHONY: dev generate test golden e2e lint check migrate docker

GO ?= go

dev:
	AWT_ENV=dev $(GO) run ./cmd/awt serve

generate:
	@if [ -n "$$(find . -name '*.templ' -not -path './legacy/*' | head -1)" ]; then $(GO) tool templ generate; else echo "no .templ files, skipping templ"; fi
	@if [ -f sqlc.yaml ]; then $(GO) tool sqlc generate; else echo "no sqlc.yaml, skipping sqlc"; fi

test:
	$(GO) test -race ./...

golden:
	@if [ -d testdata/golden ]; then $(GO) test ./internal/scoring/... -run Golden -count=1; else echo "no golden corpus yet"; fi

e2e:
	@if [ -d e2e ]; then $(GO) test ./e2e/...; else echo "no e2e suite yet"; fi

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
	@if [ -f Dockerfile ]; then docker build -t awt:dev .; else echo "no Dockerfile yet"; fi
