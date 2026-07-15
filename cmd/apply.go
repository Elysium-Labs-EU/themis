package cmd

import (
	"fmt"
	"time"

	"codeberg.org/Elysium_Labs/themis/internal/fix"
	"codeberg.org/Elysium_Labs/themis/internal/state"
	"codeberg.org/Elysium_Labs/themis/internal/ui"
	"github.com/spf13/cobra"
)

var applyForce bool

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
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — already satisfied\n", ui.TextMuted.Render("[skip]   "), ui.TextBold.Render(p.TestID))
				continue
			}
			f := fix.Registry[p.TestID]
			if f.Warn != nil {
				msg, detected, warnErr := f.Warn()
				if warnErr != nil {
					return fmt.Errorf("checking %s for warnings: %w", p.TestID, warnErr)
				}
				if detected && !applyForce {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelWarning.Render("[warn]   "), ui.TextBold.Render(p.TestID), msg)
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "           %s\n", ui.TextMuted.Render("skipped — review and rerun with --force to apply anyway"))
					continue
				}
			}
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelSuccess.Render("[applied]"), ui.TextBold.Render(p.TestID), p.Description)
		}

		if len(snap.Entries) == 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s all checks already satisfied — nothing to apply\n", ui.LabelSuccess.Render("✓"))
			return nil
		}
		if err := state.Save(state.DefaultPath, snap); err != nil {
			return fmt.Errorf("saving rollback state: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s applied %d fix(es). Rollback state saved to %s\n", ui.LabelSuccess.Render("✓"), len(snap.Entries), state.DefaultPath)
		return nil
	},
}

func init() {
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "apply fixes even if they report a warning")
}
