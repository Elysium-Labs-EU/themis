package native

import (
	"context"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
	"codeberg.org/Elysium_Labs/themis/internal/fix"
)

const unattendedUpgradesConfigPath = "/etc/apt/apt.conf.d/50unattended-upgrades"

// unattendedUpgradesFinding reports THEMIS-UNATTENDED-UPGRADES for
// finer-grained unattended-upgrades state Lynis doesn't check: whether
// leftover kernel packages get cleaned up and whether a needed reboot is
// applied automatically. This is distinct from the PKGS-7392 fix, which
// only tracks whether unattended-upgrades runs at all (paired against
// Lynis's own PKGS-7392 finding). No fix tracks this ID yet, so the
// finding carries its own Kind/Solution to stay actionable on its own.
func unattendedUpgradesFinding(ctx context.Context) (*audit.Finding, error) {
	if !packageInstalled(ctx, "unattended-upgrades") {
		return unattendedUpgradesDecision(false, false, ""), nil
	}

	content, existed, err := fix.ReadFileOrEmpty(unattendedUpgradesConfigPath)
	if err != nil {
		return nil, err
	}
	return unattendedUpgradesDecision(true, existed, string(content)), nil
}

// unattendedUpgradesDecision is the pure check behind
// unattendedUpgradesFinding. Pure — no I/O.
func unattendedUpgradesDecision(installed, configExists bool, configContent string) *audit.Finding {
	switch {
	case !installed:
		return &audit.Finding{
			TestID:      "THEMIS-UNATTENDED-UPGRADES",
			Description: "unattended-upgrades is not installed",
			Details:     "-",
			Solution:    "apt-get install unattended-upgrades",
			Kind:        "warning",
			Source:      "themis",
		}
	case !configExists:
		return &audit.Finding{
			TestID:      "THEMIS-UNATTENDED-UPGRADES",
			Description: "unattended-upgrades installed but not configured",
			Details:     unattendedUpgradesConfigPath,
			Solution:    "-",
			Kind:        "warning",
			Source:      "themis",
		}
	}

	if reason, solution := unattendedUpgradesGap(configContent); reason != "" {
		return &audit.Finding{
			TestID:      "THEMIS-UNATTENDED-UPGRADES",
			Description: reason,
			Details:     unattendedUpgradesConfigPath,
			Solution:    solution,
			Kind:        "suggestion",
			Source:      "themis",
		}
	}
	return nil
}

// unattendedUpgradesGap reports the first configuration gap found in
// 50unattended-upgrades content plus the directive that closes it, or ""
// for both if there is no gap. No fix tracks this ID, so the solution
// hint is what keeps the finding actionable. Pure — no I/O.
func unattendedUpgradesGap(content string) (reason, solution string) {
	if fix.DirectiveValue(content, "Unattended-Upgrade::Remove-Unused-Kernel-Packages") != `"true";` {
		return "unattended-upgrades does not clean up unused kernel packages",
			`set Unattended-Upgrade::Remove-Unused-Kernel-Packages "true"; in ` + unattendedUpgradesConfigPath
	}
	if fix.DirectiveValue(content, "Unattended-Upgrade::Automatic-Reboot") != `"true";` {
		return "unattended-upgrades does not automatically reboot when required",
			`set Unattended-Upgrade::Automatic-Reboot "true"; in ` + unattendedUpgradesConfigPath
	}
	return "", ""
}
