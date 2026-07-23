package cmd

import (
	"fmt"
	"strings"
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
// Each planned fix is independent of the others, so one fix failing (e.g.
// a Debian-specific fix on a platform whose package manager it hardcodes
// isn't present) does not stop the rest from being attempted — the loop
// always runs every planned fix and reports a summary at the end. A
// non-nil error is returned once the whole run is done if any fix failed,
// so callers still see apply as unsuccessful overall (issue #9).
func runApply(cmd *cobra.Command, statePath string) error {
	planned, err := resolveFixes()
	if err != nil {
		return err
	}

	snap := state.Snapshot{AppliedAt: time.Now().UTC()}
	var failures []string
	for _, p := range planned {
		if p.Satisfied {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — already satisfied\n", ui.TextMuted.Render("[skip]   "), ui.TextBold.Render(p.TestID))
			continue
		}
		f := fix.Registry[p.TestID]
		if f.Warn != nil {
			msg, detected, warnErr := f.Warn()
			if warnErr != nil {
				failures = append(failures, fmt.Sprintf("checking %s for warnings: %v", p.TestID, warnErr))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — checking for warnings: %s\n", ui.LabelError.Render("[error]  "), ui.TextBold.Render(p.TestID), warnErr)
				continue
			}
			if detected && !applyForce {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelWarning.Render("[warn]   "), ui.TextBold.Render(p.TestID), msg)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "           %s\n", ui.TextMuted.Render("skipped — review and rerun with --force to apply anyway"))
				continue
			}
		}
		revertData, applyErr := f.Apply()
		if applyErr != nil {
			// Some Fix implementations write their target file and then
			// fail at a later step (e.g. a service reload). That write is
			// real and already on disk, so a Fix.Apply() that knows this
			// may return non-nil revertData alongside the error. Record
			// it exactly like a successful entry so state.json — and a
			// later rollback — knows about the partial mutation instead
			// of losing all trace of it.
			if revertData != nil {
				snap.Entries = append(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
				if saveErr := state.Save(statePath, snap); saveErr != nil {
					failures = append(failures, fmt.Sprintf("applying %s: %v (also failed to save partial rollback state: %v)", p.TestID, applyErr, saveErr))
				} else {
					failures = append(failures, fmt.Sprintf("applying %s: %v (partial mutation recorded and revertible)", p.TestID, applyErr))
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelError.Render("[error]  "), ui.TextBold.Render(p.TestID), applyErr)
				continue
			}
			failures = append(failures, fmt.Sprintf("applying %s: %v", p.TestID, applyErr))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelError.Render("[error]  "), ui.TextBold.Render(p.TestID), applyErr)
			continue
		}
		snap.Entries = append(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
		if saveErr := state.Save(statePath, snap); saveErr != nil {
			failures = append(failures, fmt.Sprintf("applying %s: succeeded but failed to save rollback state: %v", p.TestID, saveErr))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelError.Render("[error]  "), ui.TextBold.Render(p.TestID), saveErr)
			continue
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelSuccess.Render("[applied]"), ui.TextBold.Render(p.TestID), p.Description)
	}

	if len(snap.Entries) == 0 && len(failures) == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s all checks already satisfied — nothing to apply\n", ui.LabelSuccess.Render("✓"))
		return nil
	}
	if len(snap.Entries) > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s applied %d fix(es). Rollback state saved to %s\n", ui.LabelSuccess.Render("✓"), len(snap.Entries), statePath)
	}
	if len(failures) > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %d fix(es) failed to apply:\n", ui.LabelError.Render("✗"), len(failures))
		for _, msg := range failures {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", msg)
		}
		return fmt.Errorf("%d fix(es) failed to apply: %s", len(failures), strings.Join(failures, "; "))
	}
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
