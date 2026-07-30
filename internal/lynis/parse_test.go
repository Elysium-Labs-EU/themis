package lynis

import (
	"errors"
	"strings"
	"testing"
)

// failingReader always errors on Read, simulating a truncated or
// unreadable lynis report (e.g. the file vanishing mid-scan).
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("simulated read failure") }

func TestParseReport(t *testing.T) {
	report := strings.Join([]string{
		"lynis_version=3.1.4",
		"suggestion[]=SSH-7408|Consider hardening SSH configuration|AllowTcpForwarding (set YES to NO)|-|",
		"warning[]=FIRE-4590|No active firewall found|-|-|",
		"suggestion[]=HRDN-7230|Harden the system by installing a malware scanner|-|Install a tool like rkhunter, chkrootkit, OSSEC|",
		"# a comment line, should be ignored",
		"",
	}, "\n")

	findings, err := ParseReport(strings.NewReader(report))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}

	want := []Finding{
		{TestID: "SSH-7408", Description: "Consider hardening SSH configuration", Details: "AllowTcpForwarding (set YES to NO)", Solution: "-", Kind: "suggestion"},
		{TestID: "FIRE-4590", Description: "No active firewall found", Details: "-", Solution: "-", Kind: "warning"},
		{TestID: "HRDN-7230", Description: "Harden the system by installing a malware scanner", Details: "-", Solution: "Install a tool like rkhunter, chkrootkit, OSSEC", Kind: "suggestion"},
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

func TestParseReportHandlesVeryLongLine(t *testing.T) {
	longDescription := strings.Repeat("x", 200*1024) // well past bufio.Scanner's default 64KB token cap
	report := "suggestion[]=SSH-7408|" + longDescription + "|-|-|\n"

	findings, err := ParseReport(strings.NewReader(report))
	if err != nil {
		t.Fatalf("ParseReport returned error on a long line: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Description != longDescription {
		t.Errorf("Description truncated or corrupted: got %d chars, want %d", len(findings[0].Description), len(longDescription))
	}
}

func TestParseReportPropagatesScannerError(t *testing.T) {
	// A truncated/unreadable report stream must surface as an error, not
	// be silently swallowed as "no findings".
	_, err := ParseReport(failingReader{})
	if err == nil {
		t.Fatal("expected a scanner error to propagate")
	}
}

func TestParseReportIgnoresUnknownPrefixLines(t *testing.T) {
	// A future lynis version might add a new report line kind; anything
	// that isn't suggestion[]= or warning[]= must be ignored, not
	// misparsed into a bogus finding.
	report := "some-new-field[]=value\nlynis_version=3.1.4\n"
	findings, err := ParseReport(strings.NewReader(report))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unrecognized prefixes, got %+v", findings)
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
