package cmd

import (
	"fmt"
	"slices"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [TEST-ID]",
	Short: "Revert fixes applied by `themis apply`",
	Long:  "Revert fixes applied by `themis apply`. With no argument, reverts every recorded fix (LIFO order) and clears the rollback state. With a TEST-ID, reverts only that fix and rewrites the rollback state with the remaining entries.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return runRollbackOne(cmd, state.DefaultPath, args[0])
		}
		return runRollbackAll(cmd, state.DefaultPath)
	},
}

// runRollbackAll reverts every entry in statePath (LIFO order, so later
// fixes unwind before the ones they may depend on) and clears the file.
func runRollbackAll(cmd *cobra.Command, statePath string) error {
	snap, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading rollback state: %w", err)
	}

	for _, entry := range slices.Backward(snap.Entries) {
		if err := revertEntry(cmd, entry); err != nil {
			return err
		}
	}

	if err := state.Clear(statePath); err != nil {
		return fmt.Errorf("clearing rollback state: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %d fix(es)\n", ui.LabelSuccess.Render("✓"), len(snap.Entries))
	return nil
}

// runRollbackOne reverts just the entry matching testID and rewrites
// statePath with the remaining entries — or clears it if none remain —
// instead of discarding rollback data for every other applied fix.
func runRollbackOne(cmd *cobra.Command, statePath, testID string) error {
	snap, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading rollback state: %w", err)
	}

	idx := slices.IndexFunc(snap.Entries, func(e state.Entry) bool { return e.TestID == testID })
	if idx == -1 {
		return fmt.Errorf("%s: no rollback state recorded for this TestID", testID)
	}
	if err := revertEntry(cmd, snap.Entries[idx]); err != nil {
		return err
	}

	remaining := state.Remove(snap.Entries, testID)
	if len(remaining) == 0 {
		if err := state.Clear(statePath); err != nil {
			return fmt.Errorf("clearing rollback state: %w", err)
		}
	} else {
		snap.Entries = remaining
		if err := state.Save(statePath, snap); err != nil {
			return fmt.Errorf("saving rollback state: %w", err)
		}
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %s\n", ui.LabelSuccess.Render("✓"), testID)
	return nil
}

// revertEntry looks up entry.TestID in the fix registry and calls its
// Revert, printing progress the same way for both single- and full-rollback.
// A TestID no longer in the registry is skipped, not an error — there is no
// way to revert a fix that no longer exists.
func revertEntry(cmd *cobra.Command, entry state.Entry) error {
	f, ok := fix.Registry[entry.TestID]
	if !ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — no longer registered\n", ui.LabelWarning.Render("[skip]    "), ui.TextBold.Render(entry.TestID))
		return nil
	}
	if err := f.Revert(entry.RevertData); err != nil {
		return fmt.Errorf("reverting %s: %w", entry.TestID, err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", ui.LabelSuccess.Render("[reverted]"), ui.TextBold.Render(entry.TestID))
	return nil
}
