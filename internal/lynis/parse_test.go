package lynis

import (
	"strings"
	"testing"
)

func TestParseReport(t *testing.T) {
	report := strings.Join([]string{
		"lynis_version=3.1.4",
		"suggestion[]=SSH-7408|Consider hardening SSH configuration|-|-",
		"warning[]=FIRE-4590|No active firewall found|H|-",
		"suggestion[]=PKGS-7392|Enable unattended-upgrades|M|-",
		"# a comment line, should be ignored",
		"",
	}, "\n")

	findings, err := ParseReport(strings.NewReader(report))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}

	want := []Finding{
		{TestID: "SSH-7408", Description: "Consider hardening SSH configuration", Severity: "-", Kind: "suggestion"},
		{TestID: "FIRE-4590", Description: "No active firewall found", Severity: "H", Kind: "warning"},
		{TestID: "PKGS-7392", Description: "Enable unattended-upgrades", Severity: "M", Kind: "suggestion"},
	}

	if len(findings) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(findings), len(want), findings)
	}
	for i, got := range findings {
		if got != want[i] {
			t.Errorf("finding %d: got %+v, want %+v", i, got, want[i])
		}
	}
}

func TestParseReportEmpty(t *testing.T) {
	findings, err := ParseReport(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestParseReportShortFields(t *testing.T) {
	findings, err := ParseReport(strings.NewReader("suggestion[]=TEST-0001\n"))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %+v", findings)
	}
	got := findings[0]
	want := Finding{TestID: "TEST-0001", Kind: "suggestion"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
