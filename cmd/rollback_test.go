package cmd

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// TestRollbackCmdRunEErrorsWithoutRoot is the regression test for issue
// #11: unprivileged `themis rollback` must fail fast with a clear
// root-required UserError, before ever reaching state.Load(), instead of
// whatever error state loading happens to produce.
func TestRollbackCmdRunEErrorsWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root")
	}

	buf := &bytes.Buffer{}
	rollbackCmd.SetOut(buf)
	defer rollbackCmd.SetOut(nil)

	err := rollbackCmd.RunE(rollbackCmd, nil)
	if err == nil {
		t.Fatal("expected an error when run without root")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if uerr.Hint != "sudo themis rollback" {
		t.Errorf("Hint = %q, want %q", uerr.Hint, "sudo themis rollback")
	}
}
