package cmd

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
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
	var execErr *exec.Error
	if !errors.As(err, &execErr) {
		t.Fatalf("expected an *exec.Error in the chain, got %v", err)
	}
}
