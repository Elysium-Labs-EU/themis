package cmd

import (
	"fmt"

	"codeberg.org/Elysium_Labs/themis/internal/fix"
	"codeberg.org/Elysium_Labs/themis/internal/state"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Revert the fixes applied by the last `themis apply`",
	RunE: func(cmd *cobra.Command, _ []string) error {
		snap, err := state.Load(state.DefaultPath)
		if err != nil {
			return fmt.Errorf("loading rollback state: %w", err)
		}

		// Revert in reverse order (LIFO) so later fixes unwind before
		// the ones they may depend on.
		for i := len(snap.Entries) - 1; i >= 0; i-- {
			entry := snap.Entries[i]
			f, ok := fix.Registry[entry.TestID]
			if !ok {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [skip]     %s — no longer registered\n", entry.TestID)
				continue
			}
			if err := f.Revert(entry.RevertData); err != nil {
				return fmt.Errorf("reverting %s: %w", entry.TestID, err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [reverted] %s\n", entry.TestID)
		}

		if err := state.Clear(state.DefaultPath); err != nil {
			return fmt.Errorf("clearing rollback state: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nRolled back %d fix(es).\n", len(snap.Entries))
		return nil
	},
}
