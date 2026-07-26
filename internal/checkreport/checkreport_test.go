package checkreport

import (
	"strings"
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
	if len(report.Unmatched) != 0 {
		t.Fatalf("expected no unmatched fixes, got %+v", report.Unmatched)
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
	if len(report.Unmatched) != 1 || report.Unmatched[0].TestID != "THEMIS-FAIL2BAN" {
		t.Fatalf("expected the unmatched fix to be reported, got %+v", report.Unmatched)
	}
}

func TestBuildDedupesFindingsReportedByMultipleSources(t *testing.T) {
	// Two sources merge only when they report the *same* TestID, and per-
	// source validation now requires each id to match its own source's
	// namespace — so the pair has to share a namespace. lynis and osquery
	// both use the default lynis-shaped pattern, so "SSH-7408" is valid for
	// both; openscap (xccdf_* ids) never collides with lynis's namespace,
	// so a real lynis+openscap pair wouldn't dedup here in the first place.
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "suggestion", Description: "harden ssh", Solution: "-", Source: "lynis"},
		{TestID: "SSH-7408", Kind: "suggestion", Description: "harden ssh", Solution: "-", Source: "osquery"},
	}

	report := Build(findings, nil)

	if len(report.Findings) != 1 {
		t.Fatalf("expected duplicate findings to collapse into 1, got %d", len(report.Findings))
	}
	sources := report.Findings[0].Sources
	if len(sources) != 2 || sources[0] != "lynis" || sources[1] != "osquery" {
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
	if len(report.Unmatched) != 0 {
		t.Fatalf("expected drift finding to be excluded from Unmatched, got %+v", report.Unmatched)
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

func TestBuildFlagsMalformedTestIDAsNotActionable(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "'; DROP TABLE findings;--", Kind: "warning", Description: "crafted", Solution: "run this"},
	}

	report := Build(findings, nil)

	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(report.Findings))
	}
	f := report.Findings[0]
	if !f.Malformed {
		t.Error("expected a finding with a malformed TestID to be flagged Malformed")
	}
	if f.Actionable {
		t.Error("expected a malformed finding to never be Actionable, even with Kind=warning and a Solution")
	}
}

func TestBuildFlagsUnexpectedKindAsNotActionable(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "critical-override", Description: "crafted kind", Solution: "run this"},
	}

	report := Build(findings, nil)

	if !report.Findings[0].Malformed {
		t.Error("expected an unexpected Kind to be flagged Malformed")
	}
	if report.Findings[0].Actionable {
		t.Error("expected a finding with an unexpected Kind to never be Actionable")
	}
}

func TestBuildFlagsOversizedFieldsAsNotActionable(t *testing.T) {
	huge := strings.Repeat("A", maxFieldLen+1)
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "warning", Description: huge, Solution: "-"},
	}

	report := Build(findings, nil)

	if !report.Findings[0].Malformed {
		t.Error("expected an oversized Description to be flagged Malformed")
	}
	if report.Findings[0].Actionable {
		t.Error("expected an oversized finding to never be Actionable")
	}
	if got := len(report.Findings[0].Description); got > maxFieldLen+len("…(truncated)") {
		t.Errorf("expected Description to be truncated, got %d runes", got)
	}
}

func TestBuildMalformedTestIDCannotSpoofDedupKey(t *testing.T) {
	// A TestID containing the "|" key separator could otherwise be
	// crafted to collide with an unrelated finding's dedup key
	// (TestID + "|" + Kind) and get silently merged into it.
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "suggestion", Description: "real finding", Solution: "-", Source: "lynis"},
		{TestID: "SSH-7408|suggestion", Kind: "", Description: "spoofed merge attempt", Solution: "-", Source: "attacker"},
	}

	report := Build(findings, nil)

	if len(report.Findings) != 2 {
		t.Fatalf("expected the crafted TestID to stay a separate finding, got %d: %+v", len(report.Findings), report.Findings)
	}
	real := report.Findings[0]
	if len(real.Sources) != 1 || real.Sources[0] != "lynis" {
		t.Errorf("expected the real finding's Sources untouched by the spoof attempt, got %+v", real.Sources)
	}
}

func TestBuildPairsNativeFindingWithItsFix(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "THEMIS-FAIL2BAN", Kind: "suggestion", Description: "fail2ban not active", Solution: "-", Source: "themis"},
	}
	fixes := []Fix{
		{TestID: "THEMIS-FAIL2BAN", LynisID: "THEMIS-FAIL2BAN", Description: "install and enable fail2ban"},
	}

	report := Build(findings, fixes)

	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(report.Findings))
	}
	f := report.Findings[0]
	if f.Malformed {
		t.Error("expected a native themis finding to not be flagged Malformed")
	}
	if !f.Actionable {
		t.Error("expected a native finding tracked by a fix to be actionable")
	}
	if len(f.Fixes) != 1 || f.Fixes[0].TestID != "THEMIS-FAIL2BAN" {
		t.Fatalf("expected the native fix to be attached to the finding, got %+v", f.Fixes)
	}
	if len(report.Unmatched) != 0 {
		t.Fatalf("expected the fix to be matched, not left in Unmatched, got %+v", report.Unmatched)
	}
}

func TestBuildKeepsOpenSCAPFindingActionable(t *testing.T) {
	// An OpenSCAP XCCDF rule id (internal/openscap sets Source "openscap")
	// looks nothing like Lynis's "SSH-7408" shape. It must validate against
	// openscapTestIDPattern, not the Lynis pattern — otherwise every real
	// openscap warning gets wrongly flagged Malformed and silently hidden.
	findings := []audit.Finding{
		{TestID: "xccdf_org.ssgproject.content_rule_sshd_disable_root_login", Kind: "warning", Description: "Disable SSH Root Login", Solution: "-", Source: "openscap"},
	}

	report := Build(findings, nil)

	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(report.Findings))
	}
	f := report.Findings[0]
	if f.Malformed {
		t.Error("expected an openscap xccdf-id finding to not be flagged Malformed")
	}
	if !f.Actionable {
		t.Error("expected an openscap warning to remain Actionable")
	}
}

func TestBuildTrustedFindingUnaffected(t *testing.T) {
	findings := []audit.Finding{
		{TestID: "SSH-7408", Kind: "warning", Description: "harden ssh", Solution: "-"},
	}

	report := Build(findings, nil)

	if report.Findings[0].Malformed {
		t.Error("expected a well-formed finding to not be flagged Malformed")
	}
	if !report.Findings[0].Actionable {
		t.Error("expected a well-formed warning to remain Actionable")
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
