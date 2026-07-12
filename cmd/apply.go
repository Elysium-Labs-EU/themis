package cmd

import (
	"fmt"
	"time"

	"codeberg.org/Elysium_Labs/themis/internal/fix"
	"codeberg.org/Elysium_Labs/themis/internal/state"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply all unsatisfied registered fixes and save rollback state",
	RunE: func(cmd *cobra.Command, _ []string) error {
		planned, err := resolveFixes()
		if err != nil {
			return err
		}

		snap := state.Snapshot{AppliedAt: time.Now().UTC()}
		for _, p := range planned {
			if p.Satisfied {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [skip]    %s — already satisfied\n", p.TestID)
				continue
			}
			f := fix.Registry[p.TestID]
			revertData, err := f.Apply()
			if err != nil {
				// Save state for whatever already succeeded so a
				// partial apply is still revertible.
				if len(snap.Entries) > 0 {
					if saveErr := state.Save(state.DefaultPath, snap); saveErr != nil {
						return fmt.Errorf("applying %s: %w (additionally failed to save partial rollback state: %w)", p.TestID, err, saveErr)
					}
					return fmt.Errorf("applying %s: %w (rollback state for %d earlier fix(es) saved to %s)", p.TestID, err, len(snap.Entries), state.DefaultPath)
				}
				return fmt.Errorf("applying %s: %w", p.TestID, err)
			}
			snap.Entries = append(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [applied] %s — %s\n", p.TestID, p.Description)
		}

		if len(snap.Entries) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nNothing to apply — all checks already satisfied.")
			return nil
		}
		if err := state.Save(state.DefaultPath, snap); err != nil {
			return fmt.Errorf("saving rollback state: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nApplied %d fix(es). Rollback state saved to %s.\n", len(snap.Entries), state.DefaultPath)
		return nil
	},
}
