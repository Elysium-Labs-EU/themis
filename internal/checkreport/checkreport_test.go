package checkreport

import (
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
)

func TestBuildMarksFixTrackedFindingsActionable(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "suggestion", Description: "harden ssh", Solution: "-"},
	}
	fixes := []Fix{
		{TestID: "SSH-7408-PASSWDAUTH", LynisID: "SSH-7408", Description: "disable password auth"},
	}

	report := Build(findings, fixes)

	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(report.Findings))
	}
	f := report.Findings[0]
	if !f.Actionable {
		t.Fatal("expected finding tracked by a fix to be actionable")
	}
	if len(f.Fixes) != 1 || f.Fixes[0].TestID != "SSH-7408-PASSWDAUTH" {
		t.Fatalf("expected the fix to be attached to the finding, got %+v", f.Fixes)
	}
	if len(report.Native) != 0 {
		t.Fatalf("expected no native fixes, got %+v", report.Native)
	}
}

func TestBuildMarksSolutionOnlyFindingsActionable(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "FILE-7524", Kind: "suggestion", Description: "restrict permissions", Solution: "chmod ..."},
	}

	report := Build(findings, nil)

	if !report.Findings[0].Actionable {
		t.Fatal("expected a finding with a Lynis solution hint to be actionable")
	}
}

func TestBuildMarksWarningsActionableEvenWithoutSolution(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "KRNL-5830", Kind: "warning", Description: "reboot needed", Solution: "-"},
	}

	report := Build(findings, nil)

	if !report.Findings[0].Actionable {
		t.Fatal("expected a warning-kind finding to be actionable even without a solution")
	}
}

func TestBuildHidesPlainSuggestionsWithNoFixOrSolution(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "BANN-7126", Kind: "suggestion", Description: "add banner", Solution: "-"},
	}

	report := Build(findings, nil)

	if report.Findings[0].Actionable {
		t.Fatal("expected a plain suggestion with no fix or solution to be hidden")
	}

	hidden := report.Hidden()
	if len(hidden) != 1 || hidden[0].TestID != "BANN-7126" {
		t.Fatalf("expected Hidden to return the unactionable finding, got %+v", hidden)
	}
}

func TestBuildCollectsUnmatchedFixesAsNative(t *testing.T) {
	fixes := []Fix{
		{TestID: "THEMIS-FAIL2BAN", LynisID: "", Description: "install fail2ban"},
	}

	report := Build(nil, fixes)

	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %+v", report.Findings)
	}
	if len(report.Native) != 1 || report.Native[0].TestID != "THEMIS-FAIL2BAN" {
		t.Fatalf("expected the unmatched fix to be reported as native, got %+v", report.Native)
	}
}

func TestBuildDedupesFindingsReportedByMultipleSources(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "suggestion", Description: "harden ssh", Solution: "-", Source: "lynis"},
		{TestID: "SSH-7408", Kind: "suggestion", Description: "harden ssh", Solution: "-", Source: "openscap"},
	}

	report := Build(findings, nil)

	if len(report.Findings) != 1 {
		t.Fatalf("expected duplicate findings to collapse into 1, got %d", len(report.Findings))
	}
	sources := report.Findings[0].Sources
	if len(sources) != 2 || sources[0] != "lynis" || sources[1] != "openscap" {
		t.Fatalf("expected both sources recorded, got %+v", sources)
	}
}

func TestBuildRoutesDriftFindingsSeparatelyFromGenericFindings(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "THEMIS-FAIL2BAN", Kind: "drift", Description: "fail2ban stopped running", Details: "applied 2026-01-01T00:00:00Z, no longer satisfied", Source: "osquery"},
	}

	report := Build(findings, nil)

	if len(report.Findings) != 0 {
		t.Fatalf("expected drift finding to be excluded from Findings, got %+v", report.Findings)
	}
	if len(report.Native) != 0 {
		t.Fatalf("expected drift finding to be excluded from Native, got %+v", report.Native)
	}
	if len(report.Drift) != 1 {
		t.Fatalf("expected 1 drift finding, got %+v", report.Drift)
	}
	d := report.Drift[0]
	if d.TestID != "THEMIS-FAIL2BAN" || !d.Actionable || d.Details == "" {
		t.Errorf("drift finding = %+v, want TestID=THEMIS-FAIL2BAN Actionable=true with Details set", d)
	}
	if len(d.Sources) != 1 || d.Sources[0] != "osquery" {
		t.Errorf("drift finding Sources = %+v, want [osquery]", d.Sources)
	}
}

func TestHiddenReturnsEmptyWhenAllActionable(t *testing.T) {
	report := Report{
		Findings: []Finding{
			{TestID: "A", Actionable: true},
			{TestID: "B", Actionable: true},
		},
	}

	if hidden := report.Hidden(); len(hidden) != 0 {
		t.Fatalf("expected no hidden findings, got %+v", hidden)
	}
}
