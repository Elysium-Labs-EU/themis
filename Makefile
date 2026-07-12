.PHONY: help build test test-coverage-check lint nilcheck sg fix setup ci test-linux build-orb demo-orb lynis-install-orb orb-shell clean

ORB_MACHINE ?= debian
COVERAGE_THRESHOLD ?= 49
BINARY_NAME=themis
GOBIN=./bin

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}' | sort

build: ## Build binary
	@mkdir -p $(GOBIN)
	CGO_ENABLED=0 go build -o $(GOBIN)/$(BINARY_NAME) .

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
	ssh orb "sudo /tmp/themis check"

lynis-install-orb: ## Install lynis on the orb debian VM (one-time setup)
	ssh orb "sudo apt-get update && sudo apt-get install -y lynis"

orb-shell: ## SSH into the orb debian VM
	ssh orb

clean: ## Remove build artifacts
	rm -rf $(GOBIN) coverage.out
	go clean
