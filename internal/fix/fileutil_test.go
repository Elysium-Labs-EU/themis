package fix

import (
	"strings"
	"testing"
)

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
