.PHONY: help build test test-coverage-check lint nilcheck sg fix setup ci test-linux build-orb demo-orb lynis-install-orb orb-shell clean release release-local changelog changelog-preview pre-release

ORB_MACHINE ?= debian
COVERAGE_THRESHOLD ?= 49
BINARY_NAME=themis
GOBIN=./bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
VERSION_PKG := codeberg.org/Elysium_Labs/themis/internal/buildinfo
LDFLAGS := -ldflags "-X '$(VERSION_PKG).Version=$(VERSION)' -X '$(VERSION_PKG).GitCommit=$(COMMIT)' -X '$(VERSION_PKG).BuildDate=$(BUILD_DATE)' -w -s"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}' | sort

build: ## Build binary
	@mkdir -p $(GOBIN)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(GOBIN)/$(BINARY_NAME) .

test: ## Run tests
	go test ./... -race -count=2

test-coverage-check: ## Fail if total coverage is below COVERAGE_THRESHOLD
	@go test -coverprofile=coverage.out ./... -covermode=atomic -count=1 2>&1 | grep -v "^?" || true
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/{gsub(/%/,""); print $$3}'); \
	echo "Total coverage: $${total}%"; \
	awk -v total="$${total}" -v threshold="$(COVERAGE_THRESHOLD)" \
		'BEGIN { if (total+0 < threshold+0) { print "Coverage " total "% below threshold " threshold "%"; exit 1 } }'

lint: ## Run all linters
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Run: make setup"; exit 1; }
	golangci-lint run --timeout=5m

nilcheck: ## Static nil-pointer safety analysis
	@command -v nilaway >/dev/null 2>&1 || { echo "nilaway not found. Run: make setup"; exit 1; }
	nilaway ./...

sg: ## Scan codebase with ast-grep rules (skipped until rules/ ported)
	@if [ -d rules ]; then ast-grep scan; else echo "no rules/ dir yet, skipping"; fi

fix: ## Fix go formatting
	golangci-lint fmt
	go tool fieldalignment -fix ./...

setup: ## Install dev tools (golangci-lint, nilaway) — same versions as eos
	@echo "Installing golangci-lint v2.11.0..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.11.0
	@echo "Installing nilaway..."
	go install go.uber.org/nilaway/cmd/nilaway@latest
	@echo "Setup complete."

ci: test lint sg nilcheck test-coverage-check ## Run all CI checks locally
	@echo "All CI checks passed!"

test-linux: ## Run tests on OrbStack $(ORB_MACHINE) Linux (mirrors CI, root env)
	orb run -m $(ORB_MACHINE) bash -lc "export PATH=/usr/local/go/bin:\$$PATH; cd $(PWD) && go test ./... -race -count=2"

build-orb: ## Build linux/arm64 binary on OrbStack $(ORB_MACHINE) (copies to /tmp to avoid FUSE issues)
	ssh orb "export PATH=\$$PATH:/usr/local/go/bin && rm -rf /tmp/themis-src && cp -r $(PWD) /tmp/themis-src && cd /tmp/themis-src && CC=clang go build -o /tmp/themis . && echo built"

demo-orb: build-orb ## Copy fresh binary to orb debian VM and run `themis check` as root (needs lynis installed on VM)
	ssh -t orb "sudo /tmp/themis check"

lynis-install-orb: ## Install lynis on the orb debian VM (one-time setup)
	ssh orb "sudo apt-get update && sudo apt-get install -y lynis"

orb-shell: ## SSH into the orb debian VM
	ssh orb

release-local: ## Build release binaries locally
	@echo "Building release binaries..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/themis-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/themis-linux-arm64 .
	cd dist && sha256sum themis-linux-* > sha256sums.txt
	@echo "Release binaries built in ./dist/"
	@ls -lh dist/

changelog: ## Generate CHANGELOG.md from git history
	@echo "Generating CHANGELOG.md..."
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff not found. Install: https://git-cliff.org/docs/installation"; exit 1; }
	git cliff --output CHANGELOG.md
	@echo "CHANGELOG.md updated"

changelog-preview: ## Preview unreleased changes (does not write to file)
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff not found. Install: https://git-cliff.org/docs/installation"; exit 1; }
	git cliff --unreleased

release: ## Update changelog, tag and push a release (requires TAG=v1.2.0)
	@if [ -z "$(TAG)" ]; then echo "Usage: make release TAG=v1.2.0"; exit 1; fi
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff not found. Install: https://git-cliff.org/docs/installation"; exit 1; }
	git cliff --tag $(TAG) --output CHANGELOG.md
	git add CHANGELOG.md
	git diff --cached --quiet CHANGELOG.md || git commit -m "chore: update changelog for $(TAG)"
	git push origin HEAD
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)

pre-release: ## Tag and push a pre-release (requires TAG=v1.2.0-rc.1, no changelog update)
	@if [ -z "$(TAG)" ]; then echo "Usage: make pre-release TAG=v1.2.0-rc.1"; exit 1; fi
	git tag -a $(TAG) -m "Pre-release $(TAG)"
	git push origin $(TAG)

clean: ## Remove build artifacts
	rm -rf $(GOBIN) dist/ coverage.out
	go clean
