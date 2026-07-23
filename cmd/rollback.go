package cmd

import (
	"fmt"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackForce bool

// runRollback reverts every entry in the snapshot at statePath, LIFO. An
// entry whose Fix reports RevertWarn drift (the target changed since apply)
// is skipped rather than reverted, unless force is set, so a hand-edit made
// after apply is never silently discarded. Skipped entries are re-saved to
// statePath (instead of clearing it) so a later `rollback --force` can
// still unwind them.
func runRollback(cmd *cobra.Command, statePath string, force bool) error {
	snap, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading rollback state: %w", err)
	}

	// skipped collects entries left un-reverted because they drifted since
	// apply, in original (apply) order, so a later `rollback --force` can
	// still unwind them LIFO.
	var skipped []state.Entry
	reverted := 0
	// Revert in reverse order (LIFO) so later fixes unwind before the ones
	// they may depend on.
	for i := len(snap.Entries) - 1; i >= 0; i-- {
		entry := snap.Entries[i]
		f, ok := fix.Registry[entry.TestID]
		if !ok {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — no longer registered\n", ui.LabelWarning.Render("[skip]    "), ui.TextBold.Render(entry.TestID))
			continue
		}
		if f.RevertWarn != nil {
			msg, detected, warnErr := f.RevertWarn(entry.RevertData)
			if warnErr != nil {
				return fmt.Errorf("checking %s for drift before revert: %w", entry.TestID, warnErr)
			}
			if detected && !force {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelWarning.Render("[warn]    "), ui.TextBold.Render(entry.TestID), msg)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "            %s\n", ui.TextMuted.Render("skipped — review and rerun rollback with --force to revert anyway"))
				skipped = append([]state.Entry{entry}, skipped...)
				continue
			}
		}
		if err := f.Revert(entry.RevertData); err != nil {
			return fmt.Errorf("reverting %s: %w", entry.TestID, err)
		}
		reverted++
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", ui.LabelSuccess.Render("[reverted]"), ui.TextBold.Render(entry.TestID))
	}

	if len(skipped) > 0 {
		if err := state.Save(statePath, state.Snapshot{AppliedAt: snap.AppliedAt, Entries: skipped}); err != nil {
			return fmt.Errorf("saving rollback state for %d skipped fix(es): %w", len(skipped), err)
		}
	} else if err := state.Clear(statePath); err != nil {
		return fmt.Errorf("clearing rollback state: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s rolled back %d fix(es)", ui.LabelSuccess.Render("✓"), reverted)
	if len(skipped) > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), ", %d skipped (drifted since apply)", len(skipped))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Revert the fixes applied by the last `themis apply`",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runRollback(cmd, state.DefaultPath, rollbackForce)
	},
}

func init() {
	rollbackCmd.Flags().BoolVar(&rollbackForce, "force", false, "revert fixes even if they report drift since apply")
}
