package cmd

import (
	"fmt"
	"slices"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackForce bool

var rollbackCmd = &cobra.Command{
	Use:   "rollback [TEST-ID]",
	Short: "Revert fixes applied by `themis apply`",
	Long:  "Revert fixes applied by `themis apply`. With no argument, reverts every recorded fix (LIFO order) and clears the rollback state. With a TEST-ID, reverts only that fix and rewrites the rollback state with the remaining entries.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireRoot("rollback", "revert fixes applied to system files and services"); err != nil {
			return err
		}
		if len(args) == 1 {
			return runRollbackOne(cmd, state.DefaultPath, args[0], rollbackForce)
		}
		return runRollbackAll(cmd, state.DefaultPath, rollbackForce)
	},
}

// runRollbackAll reverts every entry in statePath (LIFO order, so later
// fixes unwind before the ones they may depend on) and clears the file. An
// entry whose fix reports drift (its file changed since apply) is skipped
// rather than reverted unless force is set, and stays in statePath so a
// later `rollback --force` can still revert it.
func runRollbackAll(cmd *cobra.Command, statePath string, force bool) error {
	snap, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading rollback state: %w", err)
	}

	reverted, skipped, err := revertAllEntries(cmd, snap.Entries, force)
	if err != nil {
		return err
	}
	if err := persistRollbackAllState(statePath, snap, skipped); err != nil {
		return err
	}

	printRollbackAllSummary(cmd, reverted, len(skipped))
	return nil
}

// revertAllEntries reverts every entry (LIFO order, so later fixes unwind
// before the ones they may depend on), returning how many reverted and
// which TestIDs were skipped (drift detected, force not set).
func revertAllEntries(cmd *cobra.Command, entries []state.Entry, force bool) (reverted int, skipped map[string]bool, err error) {
	skipped = make(map[string]bool, len(entries))
	for _, entry := range slices.Backward(entries) {
		ok, revertErr := revertEntry(cmd, entry, force)
		if revertErr != nil {
			return reverted, skipped, revertErr
		}
		if !ok {
			skipped[entry.TestID] = true
			continue
		}
		reverted++
	}
	return reverted, skipped, nil
}

// persistRollbackAllState clears statePath once nothing was skipped, or
// rewrites it with just the skipped entries so a later `rollback --force`
// can still revert them.
func persistRollbackAllState(statePath string, snap state.Snapshot, skipped map[string]bool) error {
	if len(skipped) == 0 {
		if err := state.Clear(statePath); err != nil {
			return fmt.Errorf("clearing rollback state: %w", err)
		}
		return nil
	}
	remaining := make([]state.Entry, 0, len(skipped))
	for _, entry := range snap.Entries {
		if skipped[entry.TestID] {
			remaining = append(remaining, entry)
		}
	}
	snap.Entries = remaining
	if err := state.Save(statePath, snap); err != nil {
		return fmt.Errorf("saving rollback state: %w", err)
	}
	return nil
}

func printRollbackAllSummary(cmd *cobra.Command, reverted, skippedCount int) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %d fix(es)", ui.LabelSuccess.Render("✓"), reverted)
	if skippedCount > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), ", %d skipped (drift detected — rerun with --force)", skippedCount)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
}

// runRollbackOne reverts just the entry matching testID and rewrites
// statePath with the remaining entries — or clears it if none remain —
// instead of discarding rollback data for every other applied fix. If the
// fix reports drift and force isn't set, the entry is left untouched in
// statePath so a later `rollback TEST-ID --force` can still revert it.
func runRollbackOne(cmd *cobra.Command, statePath, testID string, force bool) error {
	snap, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading rollback state: %w", err)
	}

	idx := slices.IndexFunc(snap.Entries, func(e state.Entry) bool { return e.TestID == testID })
	if idx == -1 {
		return fmt.Errorf("%s: no rollback state recorded for this TestID", testID)
	}
	ok, err := revertEntry(cmd, snap.Entries[idx], force)
	if err != nil {
		return err
	}
	if !ok {
		return nil
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
// way to revert a fix that no longer exists. If the fix's RevertWarn
// reports drift (its file changed since apply) and force is false, Revert
// is not called at all — reverted is false so the caller keeps the entry in
// statePath instead of discarding the only record of how to undo it.
func revertEntry(cmd *cobra.Command, entry state.Entry, force bool) (reverted bool, err error) {
	f, ok := fix.Registry[entry.TestID]
	if !ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — no longer registered\n", ui.FixLabel("[skip]", ui.StatusWarned), ui.TextBold.Render(entry.TestID))
		return true, nil
	}
	if f.RevertWarn != nil {
		msg, detected, warnErr := f.RevertWarn(entry.RevertData)
		if warnErr != nil {
			return false, fmt.Errorf("checking %s for drift before revert: %w", entry.TestID, warnErr)
		}
		if detected && !force {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.FixLabel("[warn]", ui.StatusWarned), ui.TextBold.Render(entry.TestID), msg)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "            %s\n", ui.TextMuted.Render("skipped — review and rerun rollback with --force to revert anyway"))
			return false, nil
		}
	}
	if err := f.Revert(entry.RevertData); err != nil {
		return false, fmt.Errorf("reverting %s: %w", entry.TestID, err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", ui.FixLabel("[reverted]", ui.StatusChanged), ui.TextBold.Render(entry.TestID))
	return true, nil
}

func init() {
	rollbackCmd.Flags().BoolVar(&rollbackForce, "force", false, "revert even if the file has changed since apply")
}
