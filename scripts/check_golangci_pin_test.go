package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const golangciPinScript = "check-golangci-pin.sh"

func writeGolangciFixture(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("creating dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
}

// runGolangciPinScript invokes check-golangci-pin.sh against root and
// returns its combined output and exit error (nil on success).
func runGolangciPinScript(t *testing.T, root string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", golangciPinScript, root)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

const pinnedMakefileSnippet = `GOLANGCI_LINT_VERSION := $(shell cat .golangci-lint-version)

lint: ## Run all linters
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --timeout=5m

fix: ## Fix go formatting
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) fmt
`

const pinnedLefthookSnippet = `pre-commit:
  commands:
    format:
      run: go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(cat "$(git rev-parse --show-toplevel)/.golangci-lint-version") fmt
    lint:
      run: go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(cat "$(git rev-parse --show-toplevel)/.golangci-lint-version") run --timeout=5m
`

const pinnedWorkflowSnippet = `jobs:
  lint:
    steps:
      - name: Read golangci-lint version
        id: golangci-lint-version
        run: echo "version=$(cat .golangci-lint-version)" >> "$GITHUB_OUTPUT"
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a
        with:
          version: ${{ steps.golangci-lint-version.outputs.version }}
`

func writePinnedFixtureTree(t *testing.T, root string) {
	t.Helper()
	writeGolangciFixture(t, root, ".golangci-lint-version", "v2.12.2\n")
	writeGolangciFixture(t, root, "Makefile", pinnedMakefileSnippet)
	writeGolangciFixture(t, root, "lefthook.yml", pinnedLefthookSnippet)
	writeGolangciFixture(t, root, ".github/workflows/release.yml", pinnedWorkflowSnippet)
}

func TestCheckGolangciPinPassesOnRealRepoFiles(t *testing.T) {
	// The actual regression this task fixes: every consumer must resolve the
	// same version from .golangci-lint-version instead of hardcoding one.
	out, err := runGolangciPinScript(t, "..")
	if err != nil {
		t.Fatalf("expected the repo's golangci-lint pin to be consistent, got error: %v\noutput:\n%s", err, out)
	}
}

func TestCheckGolangciPinPassesOnPinnedFixtures(t *testing.T) {
	dir := t.TempDir()
	writePinnedFixtureTree(t, dir)

	out, err := runGolangciPinScript(t, dir)
	if err != nil {
		t.Fatalf("expected consistently pinned fixtures to pass, got error: %v\noutput:\n%s", err, out)
	}
}

func TestCheckGolangciPinFailsOnMissingVersionFile(t *testing.T) {
	dir := t.TempDir()
	writePinnedFixtureTree(t, dir)
	if err := os.Remove(filepath.Join(dir, ".golangci-lint-version")); err != nil {
		t.Fatalf("removing version file: %v", err)
	}

	out, err := runGolangciPinScript(t, dir)
	if err == nil {
		t.Fatalf("expected a missing version file to fail, got success\noutput:\n%s", out)
	}
	if want := "is missing"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the missing file (%q), got:\n%s", want, out)
	}
}

func TestCheckGolangciPinFailsOnMalformedVersion(t *testing.T) {
	dir := t.TempDir()
	writePinnedFixtureTree(t, dir)
	writeGolangciFixture(t, dir, ".golangci-lint-version", "v2.12\n")

	out, err := runGolangciPinScript(t, dir)
	if err == nil {
		t.Fatalf("expected a non vMAJOR.MINOR.PATCH version to fail, got success\noutput:\n%s", out)
	}
	if want := "exact vMAJOR.MINOR.PATCH"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the malformed version (%q), got:\n%s", want, out)
	}
}

func TestCheckGolangciPinFailsOnHardcodedWorkflowVersion(t *testing.T) {
	dir := t.TempDir()
	writePinnedFixtureTree(t, dir)
	writeGolangciFixture(t, dir, ".github/workflows/release.yml", `jobs:
  lint:
    steps:
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a
        with:
          version: v2.11
`)

	out, err := runGolangciPinScript(t, dir)
	if err == nil {
		t.Fatalf("expected a hardcoded workflow version to fail, got success\noutput:\n%s", out)
	}
	if want := "hardcodes a golangci-lint-action version"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the hardcoded workflow version (%q), got:\n%s", want, out)
	}
}

func TestCheckGolangciPinFailsOnBareMakefileInvocation(t *testing.T) {
	dir := t.TempDir()
	writePinnedFixtureTree(t, dir)
	writeGolangciFixture(t, dir, "Makefile", `lint: ## Run all linters
	golangci-lint run --timeout=5m

fix: ## Fix go formatting
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) fmt
`)

	out, err := runGolangciPinScript(t, dir)
	if err == nil {
		t.Fatalf("expected a bare golangci-lint invocation in the Makefile to fail, got success\noutput:\n%s", out)
	}
	if want := "invokes bare golangci-lint"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the bare Makefile invocation (%q), got:\n%s", want, out)
	}
}

func TestCheckGolangciPinFailsOnHardcodedLefthookVersion(t *testing.T) {
	dir := t.TempDir()
	writePinnedFixtureTree(t, dir)
	writeGolangciFixture(t, dir, "lefthook.yml", `pre-commit:
  commands:
    lint:
      run: go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --timeout=5m
`)

	out, err := runGolangciPinScript(t, dir)
	if err == nil {
		t.Fatalf("expected a hardcoded lefthook version to fail, got success\noutput:\n%s", out)
	}
	if want := "hardcodes a golangci-lint version"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the hardcoded lefthook version (%q), got:\n%s", want, out)
	}
}
