package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "themis")
	if err := os.WriteFile(path, []byte("fake"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestRunUninstallDeclinedLeavesEverything(t *testing.T) {
	dir := t.TempDir()
	exePath := writeFakeBinary(t, dir)
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := runUninstall(strings.NewReader("n\n"), buf, exePath, stateDir, false, false); err != nil {
		t.Fatalf("runUninstall: %v", err)
	}

	if _, err := os.Stat(exePath); err != nil {
		t.Errorf("expected binary to survive a declined confirmation, stat err: %v", err)
	}
}

func TestRunUninstallYesRemovesBinaryLeavesState(t *testing.T) {
	dir := t.TempDir()
	exePath := writeFakeBinary(t, dir)
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := runUninstall(strings.NewReader(""), buf, exePath, stateDir, true, false); err != nil {
		t.Fatalf("runUninstall: %v", err)
	}

	if _, err := os.Stat(exePath); !os.IsNotExist(err) {
		t.Errorf("expected binary to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(stateDir); err != nil {
		t.Errorf("expected state dir to survive without --purge, stat err: %v", err)
	}
	if !strings.Contains(buf.String(), "rm -rf") {
		t.Errorf("output = %q, want a manual-cleanup hint", buf.String())
	}
}

func TestRunUninstallPurgeRemovesState(t *testing.T) {
	dir := t.TempDir()
	exePath := writeFakeBinary(t, dir)
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := runUninstall(strings.NewReader(""), buf, exePath, stateDir, true, true); err != nil {
		t.Fatalf("runUninstall: %v", err)
	}

	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected state dir to be removed with --purge, stat err: %v", err)
	}
}

func TestRunUninstallPipedYesYesRemovesBinaryAndState(t *testing.T) {
	dir := t.TempDir()
	exePath := writeFakeBinary(t, dir)
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := runUninstall(strings.NewReader("y\ny\n"), buf, exePath, stateDir, false, false); err != nil {
		t.Fatalf("runUninstall: %v", err)
	}

	if _, err := os.Stat(exePath); !os.IsNotExist(err) {
		t.Errorf("expected binary to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected state dir to be removed when both piped answers are \"y\", stat err: %v", err)
	}
}

func TestRunUninstallMissingBinaryIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "already-gone")
	stateDir := filepath.Join(dir, "state")

	buf := &bytes.Buffer{}
	if err := runUninstall(strings.NewReader(""), buf, exePath, stateDir, true, false); err != nil {
		t.Fatalf("runUninstall on missing binary: %v", err)
	}
}
