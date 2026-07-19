package cmd

import (
	"fmt"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
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
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — no longer registered\n", ui.LabelWarning.Render("[skip]    "), ui.TextBold.Render(entry.TestID))
				continue
			}
			if err := f.Revert(entry.RevertData); err != nil {
				return fmt.Errorf("reverting %s: %w", entry.TestID, err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", ui.LabelSuccess.Render("[reverted]"), ui.TextBold.Render(entry.TestID))
		}

		if err := state.Clear(state.DefaultPath); err != nil {
			return fmt.Errorf("clearing rollback state: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %d fix(es)\n", ui.LabelSuccess.Render("✓"), len(snap.Entries))
		return nil
	},
}
