package cmd

import (
	"fmt"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [TEST-ID]",
	Short: "Revert the fixes applied by the last `themis apply`",
	Long:  "Revert the fixes applied by the last `themis apply`. With no argument, reverts every recorded fix. Given a TEST-ID, reverts only that fix and leaves the rest of the rollback state intact.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return runRollbackOne(cmd, state.DefaultPath, args[0])
		}
		return runRollbackAll(cmd, state.DefaultPath)
	},
}

// runRollbackAll reverts every entry in the rollback state, most-recently
// applied first (LIFO), then clears the state file entirely.
func runRollbackAll(cmd *cobra.Command, statePath string) error {
	snap, err := state.Load(statePath)
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

	if err := state.Clear(statePath); err != nil {
		return fmt.Errorf("clearing rollback state: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %d fix(es)\n", ui.LabelSuccess.Render("✓"), len(snap.Entries))
	return nil
}

// runRollbackOne reverts only the entry matching testID and rewrites
// statePath with the remaining entries, so an earlier or later apply's
// rollback data for other fixes survives.
func runRollbackOne(cmd *cobra.Command, statePath, testID string) error {
	snap, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading rollback state: %w", err)
	}

	var entry state.Entry
	found := false
	for _, e := range snap.Entries {
		if e.TestID == testID {
			entry = e
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no rollback state recorded for %s", testID)
	}

	f, ok := fix.Registry[testID]
	if !ok {
		return fmt.Errorf("reverting %s: fix no longer registered", testID)
	}
	if err := f.Revert(entry.RevertData); err != nil {
		return fmt.Errorf("reverting %s: %w", testID, err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", ui.LabelSuccess.Render("[reverted]"), ui.TextBold.Render(testID))

	remaining := state.Without(snap.Entries, testID)
	if len(remaining) == 0 {
		if err := state.Clear(statePath); err != nil {
			return fmt.Errorf("clearing rollback state: %w", err)
		}
	} else if err := state.Save(statePath, state.Snapshot{AppliedAt: snap.AppliedAt, Entries: remaining}); err != nil {
		return fmt.Errorf("saving rollback state: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %s\n", ui.LabelSuccess.Render("✓"), testID)
	return nil
}
