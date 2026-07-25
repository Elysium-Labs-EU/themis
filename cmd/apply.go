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
		if p.Satisfied {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — already satisfied\n", ui.TextMuted.Render("[skip]   "), ui.TextBold.Render(p.TestID))
			continue
		}
		f := fix.Registry[p.TestID]
		if f.Warn != nil {
			msg, detected, warnErr := f.Warn()
			if warnErr != nil {
				failed = append(failed, p.TestID)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — checking for warnings: %s\n", ui.LabelError.Render("[failed] "), ui.TextBold.Render(p.TestID), warnErr)
				continue
			}
			if detected && !applyForce {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelWarning.Render("[warn]   "), ui.TextBold.Render(p.TestID), msg)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "           %s\n", ui.TextMuted.Render("skipped — review and rerun with --force to apply anyway"))
				continue
			}
		}
		if f.SetTrust != nil {
			cidr, trustErr := resolveTrustedCIDR(cmd.InOrStdin(), cmd.OutOrStdout(), p.TestID, applyYes, applyTrust)
			if trustErr != nil {
				failed = append(failed, p.TestID)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — resolving trusted network: %s\n", ui.LabelError.Render("[failed] "), ui.TextBold.Render(p.TestID), trustErr)
				continue
			}
			f.SetTrust(cidr)
		}
		revertData, applyErr := f.Apply()
		if applyErr != nil {
			failed = append(failed, p.TestID)
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
					return fmt.Errorf("applying %s: %w (also failed to save partial rollback state: %w)", p.TestID, applyErr, saveErr)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s (partial mutation recorded and revertible; rollback state saved to %s)\n", ui.LabelError.Render("[failed] "), ui.TextBold.Render(p.TestID), applyErr, statePath)
				continue
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelError.Render("[failed] "), ui.TextBold.Render(p.TestID), applyErr)
			continue
		}
		snap.Entries = state.Upsert(snap.Entries, state.Entry{TestID: p.TestID, RevertData: revertData})
		applied++
		if err := state.Save(statePath, snap); err != nil {
			return fmt.Errorf("applying %s: succeeded but failed to save rollback state: %w", p.TestID, err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", ui.LabelSuccess.Render("[applied]"), ui.TextBold.Render(p.TestID), p.Description)
	}

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
		return runApply(cmd, state.DefaultPath)
	},
}

func init() {
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "apply fixes even if they report a warning")
	applyCmd.Flags().BoolVarP(&applyYes, "yes", "y", false, "skip interactive trust-network prompts, applying with no exemption (for unattended/cron runs)")
	applyCmd.Flags().StringVar(&applyTrust, "trust", "", "CIDR to exempt from fixes that can affect trusted networks/IPs (e.g. fail2ban ignoreip), non-interactive")
}
