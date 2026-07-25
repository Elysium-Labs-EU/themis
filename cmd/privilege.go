package cmd

import (
	"fmt"
	"os"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// requireRoot returns a UserError when the process isn't running as root,
// mirroring the check internal/lynis/run.go already does for `check`.
// apply/rollback mutate system files and services, so they need the same
// gate checked first — before resolving any fix — rather than surfacing
// whatever downstream permission or PATH error a fix happens to hit first.
func requireRoot(subcommand, action string) error {
	if os.Geteuid() != 0 {
		return &ui.UserError{
			Err:  fmt.Errorf("themis %s requires root to %s", subcommand, action),
			Hint: "sudo themis " + subcommand,
		}
	}
	return nil
}
