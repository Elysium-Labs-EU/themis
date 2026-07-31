package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// TestRequireRootErrorsForNonRootProcess covers the common case: test
// suites (and CI) run as an unprivileged user, so requireRoot must return
// a *ui.UserError naming the subcommand and a "sudo themis <subcommand>"
// hint rather than a bare error.
func TestRequireRootErrorsForNonRootProcess(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; skipping the non-root branch")
	}

	err := requireRoot("apply", "apply fixes")
	if err == nil {
		t.Fatal("expected an error when not running as root")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if !strings.Contains(uerr.Error(), "apply") || !strings.Contains(uerr.Error(), "apply fixes") {
		t.Errorf("error = %q, want it to mention the subcommand and action", uerr.Error())
	}
	if uerr.Hint != "sudo themis apply" {
		t.Errorf("Hint = %q, want %q", uerr.Hint, "sudo themis apply")
	}
}

// TestRequireRootAllowsRootProcess pins the other branch: requireRoot
// returns nil when the effective UID is 0, mirroring the same check
// internal/lynis/run.go already applies for `check`.
func TestRequireRootAllowsRootProcess(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("not running as root; skipping the root branch")
	}
	if err := requireRoot("apply", "apply fixes"); err != nil {
		t.Fatalf("requireRoot: %v, want nil when running as root", err)
	}
}
