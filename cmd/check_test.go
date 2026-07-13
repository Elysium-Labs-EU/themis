package cmd

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"

	"codeberg.org/Elysium_Labs/themis/internal/ui"
)

func TestCheckCmdRunEErrorsWithoutLynisBinary(t *testing.T) {
	if _, err := exec.LookPath("lynis"); err == nil {
		t.Skip("lynis is installed on this host; skipping the missing-binary path")
	}

	buf := &bytes.Buffer{}
	checkCmd.SetOut(buf)
	checkCmd.SetContext(context.Background())
	defer checkCmd.SetOut(nil)

	err := checkCmd.RunE(checkCmd, nil)
	if err == nil {
		t.Fatal("expected an error when the lynis binary is missing")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if uerr.Hint == "" {
		t.Error("expected a hint pointing at how to install lynis")
	}
}
