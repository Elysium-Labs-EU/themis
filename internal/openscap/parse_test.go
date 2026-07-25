package openscap

import (
	"strings"
	"testing"
)

func TestParseOutput(t *testing.T) {
	output := strings.Join([]string{
		"Title\tEnsure Firewall is Enabled",
		"Rule\txccdf_org.ssgproject.content_rule_firewall_enabled",
		"Ident\tCCE-27310-4",
		"Result\tfail",
		"",
		"Title\tDisable Root Login",
		"Rule\txccdf_org.ssgproject.content_rule_sshd_disable_root_login",
		"Result\tpass",
		"",
		"Title\tCheck Password Hashing",
		"Rule\txccdf_org.ssgproject.content_rule_set_password_hashing_algorithm",
		"Result\terror",
		"",
	}, "\n")

	findings, err := ParseOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseOutput returned error: %v", err)
	}

	want := []Finding{
		{TestID: "xccdf_org.ssgproject.content_rule_firewall_enabled", Description: "Ensure Firewall is Enabled", Details: "CCE-27310-4", Solution: "-", Kind: "warning"},
		{TestID: "xccdf_org.ssgproject.content_rule_set_password_hashing_algorithm", Description: "Check Password Hashing", Details: "-", Solution: "-", Kind: "suggestion"},
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

func TestParseOutputEmpty(t *testing.T) {
	findings, err := ParseOutput(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseOutput returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestParseOutputSkipsAllCompliantResults(t *testing.T) {
	for _, result := range []string{"pass", "notapplicable", "notselected", "informational", "fixed"} {
		output := "Title\tSome rule\nRule\txccdf_test_rule\nResult\t" + result + "\n"
		findings, err := ParseOutput(strings.NewReader(output))
		if err != nil {
			t.Fatalf("ParseOutput(%s) returned error: %v", result, err)
		}
		if len(findings) != 0 {
			t.Errorf("result %q: expected no findings, got %+v", result, findings)
		}
	}
}

func TestParseOutputNoTrailingBlankLine(t *testing.T) {
	output := "Title\tNo Trailing Newline Block\nRule\txccdf_test_rule\nResult\tfail"

	findings, err := ParseOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseOutput returned error: %v", err)
	}
	if len(findings) != 1 || findings[0].TestID != "xccdf_test_rule" {
		t.Fatalf("expected the final block (no trailing blank line) to be parsed, got %+v", findings)
	}
}

func TestParseOutputHandlesSpaceSeparatedFields(t *testing.T) {
	// oscap's real stdout pads with spaces, not necessarily a tab.
	output := "Title   Space Separated\nRule    xccdf_test_rule\nResult  fail\n"

	findings, err := ParseOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseOutput returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %+v", findings)
	}
	if findings[0].Description != "Space Separated" || findings[0].TestID != "xccdf_test_rule" {
		t.Errorf("got %+v", findings[0])
	}
}

func TestSplitLabelValue(t *testing.T) {
	cases := []struct {
		line, label, value string
	}{
		{"Result\tfail", "Result", "fail"},
		{"Title   Ensure Firewall is Enabled", "Title", "Ensure Firewall is Enabled"},
		{"Rule", "Rule", ""},
		{"  Result  fail", "Result", "fail"},
	}
	for _, c := range cases {
		label, value := splitLabelValue(c.line)
		if label != c.label || value != c.value {
			t.Errorf("splitLabelValue(%q) = (%q, %q), want (%q, %q)", c.line, label, value, c.label, c.value)
		}
	}
}
