package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

var (
	applyForce bool
	applyYes   bool
	applyTrust string
)

// applyOutcome is what happened when runApply tried to bring one planned
// fix to satisfied.
type applyOutcome int

const (
	outcomeSkipped applyOutcome = iota
	outcomeApplied
	outcomeFailed
)

// runApply applies every unsatisfied planned fix and persists rollback
// state to statePath. State is saved after every single successful Apply
// — not just once at the end of the loop — so a `kill -9` or SIGINT that
// lands mid-loop can, at worst, lose the one fix currently in flight
// rather than every fix already applied. kill -9 can't be trapped by a
// signal handler, so incremental durability of already-succeeded work is
// the only way to make that case safe.
//
// A single fix failing (e.g. a Debian-specific fix shelling out to a
// binary that doesn't exist on this platform) does not abort the run:
// each fix is independent, so the loop always continues to the rest and
// reports every failure in a summary at the end (issue #9 — Alpine has no
// apt-get, but that must not block platform-neutral fixes like KRNL-6000
// from being attempted). A state.Save I/O failure is different: it means
// rollback data can no longer be trusted to persist for anything applied
// after it, so that still aborts the run immediately.
func runApply(cmd *cobra.Command, statePath string) error {
	planned, err := resolveFixes()
	if err != nil {
		return err
	}

	existing, err := state.LoadOrEmpty(statePath)
	if err != nil {
		return fmt.Errorf("loading existing rollback state: %w", err)
	}
	snap := state.Snapshot{AppliedAt: time.Now().UTC(), Entries: existing.Entries}
	applied := 0
	var failed []string
	for _, p := range planned {
		outcome, applyErr := applyPlannedFix(cmd, statePath, p, &snap)
		if applyErr != nil {
			return applyErr
		}
		switch outcome {
		case outcomeApplied:
			applied++
		case outcomeFailed:
			failed = append(failed, p.TestID)
		case outcomeSkipped:
		}
	}

	return reportApplyOutcome(cmd, statePath, applied, failed)
}

// applyPlannedFix brings a single planned fix to satisfied — skipping it
// (already satisfied, or a Warn the operator hasn't --force'd past),
// resolving a trust exemption if the fix needs one, and applying — printing
// progress the same way for every case. Only a state.Save I/O failure
// returns a non-nil error; every other outcome (including an Apply failure)
// is reported via the returned applyOutcome instead, since one fix failing
// must not abort the rest of the run.
func applyPlannedFix(cmd *cobra.Command, statePath string, p PlannedFix, snap *state.Snapshot) (applyOutcome, error) {
	if p.Satisfied {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — already satisfied\n", ui.FixLabel("[ok]", ui.StatusSatisfied), ui.TextBold.Render(p.TestID))
		return outcomeSkipped, nil
	}
	f := fix.Registry[p.TestID]

	if outcome, stop := checkApplyWarn(cmd, f.Warn, p.TestID); stop {
		return outcome, nil
	}

	if f.SetTrust != nil {
		cidr, trustErr := resolveTrustedCIDR(cmd.InOrStdin(), cmd.OutOrStdout(), p.TestID, applyYes, applyTrust)
		if trustErr != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — resolving trusted network: %s\n", ui.FixLabel("[failed]", ui.StatusFailed), ui.TextBold.Render(p.TestID), trustErr)
			return outcomeFailed, nil
		}
		f.SetTrust(cidr)
	}

	return applyAndSave(cmd, statePath, f.Apply, p, snap)
}

// checkApplyWarn runs warn (a Fix's Warn field, if set) and reports whether
// applyPlannedFix should stop here without applying: stop is true when warn
// itself failed (outcome outcomeFailed) or detected something the operator
// hasn't --force'd past (outcome outcomeSkipped). stop is false — proceed to
// apply — when there's nothing to warn about.
func checkApplyWarn(cmd *cobra.Command, warn func() (string, bool, error), testID string) (outcome applyOutcome, stop bool) {
	if warn == nil {
		return outcomeSkipped, false
	}
	msg, detected, warnErr := warn()
	if warnErr != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — checking for warnings: %s\n", ui.FixLabel("[failed]", ui.StatusFailed), ui.TextBold.Render(testID), warnErr)
		return outcomeFailed, true
	}
	if detected && !applyForce {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.FixLabel("[warn]", ui.StatusWarned), ui.TextBold.Render(testID), msg)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "           %s\n", ui.TextMuted.Render("skipped — review and rerun with --force to apply anyway"))
		return outcomeSkipped, true
	}
	return outcomeSkipped, false
}

// applyAndSave calls apply (a Fix's Apply field) and persists the resulting
// revert data (partial or complete) to statePath, printing progress the
// same way runApply always has.
func applyAndSave(cmd *cobra.Command, statePath string, apply func() ([]byte, error), p PlannedFix, snap *state.Snapshot) (applyOutcome, error) {
	revertData, applyErr := apply()
	if applyErr != nil {
		return recordFailedApply(cmd, statePath, p.TestID, applyErr, revertData, snap)
	}
	snap.Entries = state.Upsert(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
	if saveErr := state.Save(statePath, *snap); saveErr != nil {
		return outcomeFailed, fmt.Errorf("applying %s: succeeded but failed to save rollback state: %w", p.TestID, saveErr)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.FixLabel("[applied]", ui.StatusChanged), ui.TextBold.Render(p.TestID), p.Description)
	return outcomeApplied, nil
}

// recordFailedApply reports a failed f.Apply call. Some Fix implementations
// write their target file and then fail at a later step (e.g. a service
// reload): that write is real and already on disk, so a Fix.Apply() that
// knows this may return non-nil revertData alongside the error. Record it
// exactly like a successful entry so state.json — and a later rollback —
// knows about the partial mutation instead of losing all trace of it.
func recordFailedApply(cmd *cobra.Command, statePath, testID string, applyErr error, revertData []byte, snap *state.Snapshot) (applyOutcome, error) {
	if revertData == nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.FixLabel("[failed]", ui.StatusFailed), ui.TextBold.Render(testID), applyErr)
		return outcomeFailed, nil
	}
	snap.Entries = state.Upsert(snap.Entries, state.Entry{TestID: testID, RevertData: revertData})
	if saveErr := state.Save(statePath, *snap); saveErr != nil {
		return outcomeFailed, fmt.Errorf("applying %s: %w (also failed to save partial rollback state: %w)", testID, applyErr, saveErr)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s (partial mutation recorded and revertible; rollback state saved to %s)\n", ui.FixLabel("[failed]", ui.StatusFailed), ui.TextBold.Render(testID), applyErr, statePath)
	return outcomeFailed, nil
}

// reportApplyOutcome prints the run summary and turns any failures into the
// error runApply returns.
func reportApplyOutcome(cmd *cobra.Command, statePath string, applied int, failed []string) error {
	if len(failed) > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s %d fix(es) applied, %d fix(es) failed: %s\n", ui.LabelError.Render("✗"), applied, len(failed), strings.Join(failed, ", "))
		return fmt.Errorf("%d fix(es) failed to apply: %s (rollback state for anything that did succeed is already saved to %s)", len(failed), strings.Join(failed, ", "), statePath)
	}
	if applied == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s all checks already satisfied — nothing to apply\n", ui.LabelSuccess.Render("✓"))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s applied %d fix(es). Rollback state saved to %s\n", ui.LabelSuccess.Render("✓"), applied, statePath)
	return nil
}

// resolveTrustedCIDR decides which CIDR (if any) a TrustNetwork-affecting
// fix (e.g. fail2ban's ignoreip allowlist) should exempt from its own
// enforcement, so applying it can't ban the operator's own connection.
// --trust wins outright (also covers unattended/cron runs); --yes applies
// with no exemption; otherwise the operator is prompted interactively.
func resolveTrustedCIDR(in io.Reader, out io.Writer, testID string, yes bool, trustFlag string) (string, error) {
	if trustFlag != "" {
		cidr, err := normalizeCIDR(trustFlag)
		if err != nil {
			return "", fmt.Errorf("--trust %q: %w", trustFlag, err)
		}
		return cidr, nil
	}
	if yes {
		return "", nil
	}

	current, hasCurrent := fix.CurrentConnectionCIDR()
	_, _ = fmt.Fprintf(out, "  %s %s can ban an address after repeated failed logins — including yours, if you mistype a password while managing this host remotely.\n",
		ui.LabelWarning.Render("?"), ui.TextBold.Render(testID))
	if hasCurrent {
		_, _ = fmt.Fprintf(out, "    Exempt a trusted network from ever being banned? [n]o (default) / [c]urrent connection (%s) / <CIDR>\n  > ", current)
	} else {
		_, _ = fmt.Fprint(out, "    Exempt a trusted network from ever being banned? [n]o (default) / <CIDR>\n  > ")
	}
	line, _ := bufio.NewReader(in).ReadString('\n')
	return resolveTrustAnswer(line, current)
}

// resolveTrustAnswer turns a raw prompt answer into the CIDR to exempt (""
// for no exemption), given the detected current-connection CIDR ("" if
// none). Pure — no I/O.
func resolveTrustAnswer(answer, currentCIDR string) (string, error) {
	answer = strings.TrimSpace(strings.ToLower(answer))
	switch answer {
	case "", "n", "no":
		return "", nil
	case "c", "current":
		if currentCIDR == "" {
			return "", errors.New("no current SSH connection detected (SSH_CONNECTION unset) — rerun with --trust <cidr> instead")
		}
		return currentCIDR, nil
	default:
		return normalizeCIDR(answer)
	}
}

// normalizeCIDR accepts either a bare IP (widened to a host-only /32 or
// /128 route) or an already-valid CIDR, and returns canonical CIDR form.
// Pure — no I/O.
func normalizeCIDR(s string) (string, error) {
	if _, _, err := net.ParseCIDR(s); err == nil {
		return s, nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return "", fmt.Errorf("%q is not a valid IP or CIDR", s)
	}
	if ip.To4() != nil {
		return s + "/32", nil
	}
	return s + "/128", nil
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply all unsatisfied registered fixes and save rollback state",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireRoot("apply", "apply fixes to system files and services"); err != nil {
			return err
		}
		return runApply(cmd, state.DefaultPath)
	},
}

func init() {
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "apply fixes even if they report a warning")
	applyCmd.Flags().BoolVarP(&applyYes, "yes", "y", false, "skip interactive trust-network prompts, applying with no exemption (for unattended/cron runs)")
	applyCmd.Flags().StringVar(&applyTrust, "trust", "", "CIDR to exempt from fixes that can affect trusted networks/IPs (e.g. fail2ban ignoreip), non-interactive")
}
