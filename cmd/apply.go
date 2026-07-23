package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

var applyForce bool

// runApply applies every unsatisfied planned fix and persists rollback
// state to statePath. State is saved after every single successful Apply
// — not just once at the end of the loop — so a `kill -9` or SIGINT that
// lands mid-loop can, at worst, lose the one fix currently in flight
// rather than every fix already applied. kill -9 can't be trapped by a
// signal handler, so incremental durability of already-succeeded work is
// the only way to make that case safe.
//
// Entries already on disk from an earlier `apply` run are loaded first and
// merged by TestID (state.Upsert) rather than discarded, so this run's
// state.Save calls never silently drop rollback data an earlier run wrote.
func runApply(cmd *cobra.Command, statePath string) error {
	planned, err := resolveFixes()
	if err != nil {
		return err
	}

	existing, err := state.Load(statePath)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		existing = state.Snapshot{}
	default:
		return fmt.Errorf("loading existing rollback state: %w", err)
	}

	snap := state.Snapshot{AppliedAt: time.Now().UTC(), Entries: existing.Entries}
	var appliedThisRun int
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
			// Some Fix implementations write their target file and then
			// fail at a later step (e.g. a service reload). That write is
			// real and already on disk, so a Fix.Apply() that knows this
			// may return non-nil revertData alongside the error. Record
			// it exactly like a successful entry so state.json — and a
			// later rollback — knows about the partial mutation instead
			// of losing all trace of it.
			if revertData != nil {
				snap.Entries = state.Upsert(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
				if saveErr := state.Save(statePath, snap); saveErr != nil {
					return fmt.Errorf("applying %s: %w (also failed to save partial rollback state: %w)", p.TestID, err, saveErr)
				}
				return fmt.Errorf("applying %s: %w (partial mutation recorded and revertible; rollback state saved to %s)", p.TestID, err, statePath)
			}
			// Whatever already succeeded was saved to statePath as soon
			// as it happened, so it's already revertible.
			if appliedThisRun > 0 {
				return fmt.Errorf("applying %s: %w (rollback state for %d earlier fix(es) already saved to %s)", p.TestID, err, appliedThisRun, statePath)
			}
			return fmt.Errorf("applying %s: %w", p.TestID, err)
		}
		snap.Entries = state.Upsert(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
		appliedThisRun++
		if err := state.Save(statePath, snap); err != nil {
			return fmt.Errorf("applying %s: succeeded but failed to save rollback state: %w", p.TestID, err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelSuccess.Render("[applied]"), ui.TextBold.Render(p.TestID), p.Description)
	}

	if appliedThisRun == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s all checks already satisfied — nothing to apply\n", ui.LabelSuccess.Render("✓"))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s applied %d fix(es). Rollback state saved to %s\n", ui.LabelSuccess.Render("✓"), appliedThisRun, statePath)
	return nil
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply all unsatisfied registered fixes and save rollback state",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runApply(cmd, state.DefaultPath)
	},
}

func init() {
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "apply fixes even if they report a warning")
}
