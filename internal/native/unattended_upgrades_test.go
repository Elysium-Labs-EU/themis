package native

import (
	"context"
	"errors"
	"testing"
)

func TestUnattendedUpgradesDecision(t *testing.T) {
	fullyConfigured := "Unattended-Upgrade::Remove-Unused-Kernel-Packages \"true\";\n" +
		"Unattended-Upgrade::Automatic-Reboot \"true\";\n"

	cases := []struct {
		name          string
		configContent string
		installed     bool
		configExists  bool
		wantFinding   bool
	}{
		{name: "not installed", installed: false, configExists: false, configContent: "", wantFinding: true},
		{name: "installed but not configured", installed: true, configExists: false, configContent: "", wantFinding: true},
		{name: "missing kernel cleanup", installed: true, configExists: true, configContent: "", wantFinding: true},
		{
			name: "missing auto reboot", installed: true, configExists: true,
			configContent: "Unattended-Upgrade::Remove-Unused-Kernel-Packages \"true\";\n", wantFinding: true,
		},
		{name: "fully configured", installed: true, configExists: true, configContent: fullyConfigured, wantFinding: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := unattendedUpgradesDecision(tc.installed, tc.configExists, tc.configContent)
			if (f != nil) != tc.wantFinding {
				t.Fatalf("unattendedUpgradesDecision(...) = %v, want finding=%v", f, tc.wantFinding)
			}
			if f != nil && f.TestID != "THEMIS-UNATTENDED-UPGRADES" {
				t.Errorf("TestID = %q, want THEMIS-UNATTENDED-UPGRADES", f.TestID)
			}
			// No fix tracks THEMIS-UNATTENDED-UPGRADES. checkreport.Build
			// treats Kind=="warning" as actionable regardless of
			// Solution, but a Kind=="suggestion" finding needs a real
			// solution hint or it gets hidden by default.
			if f != nil && f.Kind == "suggestion" && (f.Solution == "" || f.Solution == "-") {
				t.Errorf("Solution = %q, want a real hint for a suggestion-kind finding with no tracking fix", f.Solution)
			}
		})
	}
}

func TestUnattendedUpgradesGap(t *testing.T) {
	if reason, solution := unattendedUpgradesGap(""); reason == "" || solution == "" {
		t.Errorf("expected a gap reason and solution for empty config, got reason=%q solution=%q", reason, solution)
	}
	fullyConfigured := "Unattended-Upgrade::Remove-Unused-Kernel-Packages \"true\";\n" +
		"Unattended-Upgrade::Automatic-Reboot \"true\";\n"
	if reason, solution := unattendedUpgradesGap(fullyConfigured); reason != "" || solution != "" {
		t.Errorf("expected no gap, got reason=%q solution=%q", reason, solution)
	}
}

func TestUnattendedUpgradesFinding(t *testing.T) {
	// unattendedUpgradesConfigPath is a fixed absolute host path that
	// doesn't exist on a dev/CI box, so the installed branch
	// deterministically falls through to "installed but not configured".
	cases := []struct {
		name            string
		dpkgQuery       stubResult
		wantDescription string
	}{
		{"not installed", stubResult{err: errors.New("no such package")}, "unattended-upgrades is not installed"},
		{"installed but unconfigured host", stubResult{out: "install ok installed"}, "unattended-upgrades installed but not configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := stubRunner(t, map[string]stubResult{"dpkg-query": tc.dpkgQuery})

			f, err := unattendedUpgradesFinding(context.Background(), runner)
			if err != nil {
				t.Fatalf("unattendedUpgradesFinding: %v", err)
			}
			if f == nil {
				t.Fatal("expected a finding")
			}
			if f.TestID != "THEMIS-UNATTENDED-UPGRADES" {
				t.Errorf("TestID = %q, want THEMIS-UNATTENDED-UPGRADES", f.TestID)
			}
			if f.Description != tc.wantDescription {
				t.Errorf("Description = %q, want %q", f.Description, tc.wantDescription)
			}
		})
	}
}
