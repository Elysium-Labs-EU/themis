// Package openscap wraps OpenSCAP's `oscap xccdf eval` for CIS/DISA
// benchmark scans (typically against SCAP Security Guide / oscap-ssg
// content) and parses its rule-by-rule results into structured findings.
// It plugs into audit.Run as one more audit.Source alongside Lynis.
//
// OpenSCAP rule IDs (e.g.
// "xccdf_org.ssgproject.content_rule_sshd_disable_root_login") are a
// different shape from Lynis test IDs (e.g. "SSH-7408"), so they never
// collide in fix.Registry, which matches a Fix to a finding by exact
// TestID/LynisID string equality. No themis fix currently targets an
// OpenSCAP rule ID; OpenSCAP findings surface as themis-unmatched
// (actionable only via their "warning" kind) until one is registered.
package openscap

import (
	"bufio"
	"io"
	"strings"
)

// Finding is one non-compliant rule result from an OpenSCAP xccdf eval run.
type Finding struct {
	TestID      string
	Description string
	// Details carries the rule's CCE identifier when oscap reported one,
	// else "-".
	Details string
	// Solution is always "-": oscap's default eval output gives no
	// remediation hint (unlike Lynis), only the rule result itself.
	Solution string
	Kind     string // "warning" (result: fail) or "suggestion" (result: error/unknown/notchecked)
}

// compliantResults are xccdf rule-result values that indicate the rule is
// not a finding: either it passed, doesn't apply, or was never in scope.
var compliantResults = map[string]bool{
	"pass":          true,
	"notapplicable": true,
	"notselected":   true,
	"informational": true,
	"fixed":         true,
}

// resultBlock accumulates one Title/Rule/Ident/Result block from oscap's
// output before it's converted into a Finding.
type resultBlock struct {
	title  string
	rule   string
	ident  string
	result string
}

// finding converts b into a Finding, or reports ok=false when b has no
// rule/result (a blank line between blocks, or trailing oscap chatter) or
// its result is compliant and therefore not a finding.
func (b resultBlock) finding() (f Finding, ok bool) {
	if b.rule == "" || b.result == "" || compliantResults[b.result] {
		return Finding{}, false
	}
	kind := "suggestion"
	if b.result == "fail" {
		kind = "warning"
	}
	details := "-"
	if b.ident != "" {
		details = b.ident
	}
	return Finding{
		TestID:      b.rule,
		Description: b.title,
		Details:     details,
		Solution:    "-",
		Kind:        kind,
	}, true
}

// ParseOutput reads oscap's default `xccdf eval` stdout — blocks of
// "Title\t...", "Rule\t...", "Ident\t...", "Result\t..." lines separated
// by blank lines — and extracts one Finding per non-compliant rule.
func ParseOutput(r io.Reader) ([]Finding, error) {
	var findings []Finding
	var cur resultBlock

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if f, ok := cur.finding(); ok {
				findings = append(findings, f)
			}
			cur = resultBlock{}
			continue
		}

		label, value := splitLabelValue(line)
		switch label {
		case "Title":
			cur.title = value
		case "Rule":
			cur.rule = value
		case "Ident":
			cur.ident = value
		case "Result":
			cur.result = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if f, ok := cur.finding(); ok {
		findings = append(findings, f)
	}

	return findings, nil
}

// splitLabelValue splits an oscap output line ("Result  fail") into its
// label and value, tolerating either tabs or runs of spaces as the
// separator. Pure — no I/O.
func splitLabelValue(line string) (label, value string) {
	trimmed := strings.TrimLeft(line, " \t")
	idx := strings.IndexAny(trimmed, " \t")
	if idx < 0 {
		return trimmed, ""
	}
	return trimmed[:idx], strings.TrimSpace(trimmed[idx:])
}
