package cmd

import (
	"fmt"
	"os"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// requireRootEuid returns a UserError when euid isn't 0, pointing the user
// at the sudo invocation for cmdName. Pure — the os.Geteuid() read happens
// at the call site in requireRoot, not here.
func requireRootEuid(euid int, cmdName string) error {
	if euid != 0 {
		return &ui.UserError{
			Err:  fmt.Errorf("themis %s requires root", cmdName),
			Hint: fmt.Sprintf("sudo themis %s", cmdName),
		}
	}
	return nil
}

// requireRoot gates a command behind root, matching the explicit check
// `check` already gets for free via lynis.Audit (internal/lynis/run.go).
// Call it first in a command's RunE, before any fix resolution, so an
// unprivileged run fails with a clear "forgot sudo" message instead of
// whatever unrelated error the underlying fix logic happens to hit first.
func requireRoot(cmdName string) error {
	return requireRootEuid(os.Geteuid(), cmdName)
}
