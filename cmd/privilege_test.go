package cmd

import (
	"errors"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// TestRequireRootEuid is the regression test for issue #11: apply/rollback
// had no equivalent of check's root gate, so an unprivileged run fell
// straight into fix resolution and surfaced whatever unrelated error fired
// first instead of a clear "forgot sudo" message.
func TestRequireRootEuid(t *testing.T) {
	if err := requireRootEuid(0, "apply"); err != nil {
		t.Fatalf("requireRootEuid(0, ...) = %v, want nil", err)
	}

	err := requireRootEuid(1000, "apply")
	if err == nil {
		t.Fatal("requireRootEuid(1000, ...) = nil, want a UserError")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if uerr.Hint != "sudo themis apply" {
		t.Errorf("Hint = %q, want %q", uerr.Hint, "sudo themis apply")
	}
	if uerr.Error() != "themis apply requires root" {
		t.Errorf("Error() = %q, want %q", uerr.Error(), "themis apply requires root")
	}
}

func TestRequireRootEuidUsesCmdNameForRollback(t *testing.T) {
	err := requireRootEuid(1000, "rollback")
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if uerr.Hint != "sudo themis rollback" {
		t.Errorf("Hint = %q, want %q", uerr.Hint, "sudo themis rollback")
	}
}
