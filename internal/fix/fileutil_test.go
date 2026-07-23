package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevertDriftedNoDriftWhenContentMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("applied content\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	msg, detected, err := revertDrifted(path, "applied content\n")
	if err != nil {
		t.Fatalf("revertDrifted: %v", err)
	}
	if detected || msg != "" {
		t.Fatalf("revertDrifted = (%q, %v), want (\"\", false)", msg, detected)
	}
}

func TestRevertDriftedDetectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("applied content\nplus an admin edit\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	msg, detected, err := revertDrifted(path, "applied content\n")
	if err != nil {
		t.Fatalf("revertDrifted: %v", err)
	}
	if !detected {
		t.Fatal("expected drift to be detected")
	}
	if !strings.Contains(msg, path) {
		t.Fatalf("expected message to reference %q, got %q", path, msg)
	}
}

func TestRevertDriftedNoDriftWhenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	_, detected, err := revertDrifted(path, "applied content\n")
	if err != nil {
		t.Fatalf("revertDrifted: %v", err)
	}
	if detected {
		t.Fatal("expected no drift when the file doesn't exist — nothing left to discard")
	}
}

func TestRunCmdSuccess(t *testing.T) {
	if err := runCmd("true"); err != nil {
		t.Fatalf("runCmd(true): %v", err)
	}
}

func TestRunCmdFailure(t *testing.T) {
	if err := runCmd("false"); err == nil {
		t.Fatal("expected runCmd(false) to return an error")
	}
}

func TestRunCmdOutput(t *testing.T) {
	out, err := runCmdOutput("echo", "hello-themis")
	if err != nil {
		t.Fatalf("runCmdOutput: %v", err)
	}
	if !strings.Contains(out, "hello-themis") {
		t.Fatalf("expected output to contain %q, got %q", "hello-themis", out)
	}
}

func TestPackageInstalledFalseForUnknownPackage(t *testing.T) {
	if packageInstalled("definitely-not-a-real-package-xyz123") {
		t.Fatal("expected unknown package to report not installed")
	}
}
