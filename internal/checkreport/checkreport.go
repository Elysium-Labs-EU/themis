// Package checkreport merges Lynis findings with themis's tracked fixes
// into one report, shared by the human (cmd/check.go) and machine
// (cmd/api_check.go) output paths.
package checkreport

import "codeberg.org/Elysium_Labs/themis/internal/lynis"

// Fix is a themis-tracked fix, resolved against the Lynis test ID it
// addresses.
type Fix struct {
	TestID      string
	LynisID     string
	Description string
	Satisfied   bool
}

// Finding is one Lynis finding merged with any themis fixes that track it.
type Finding struct {
	TestID      string
	Kind        string
	Description string
	// Solution is Lynis's own remediation hint, e.g. a command to run
	// or setting to change. "-" when Lynis gave none.
	Solution string
	Fixes    []Fix
	// Actionable is false when nothing points to a next step: no themis
	// fix tracks it, Lynis gave no solution hint, and it's not a
	// warning (Lynis's own higher-severity bucket) — safe to hide by
	// default.
	Actionable bool
}

// Report is the full merge: every Lynis finding (tagged actionable or
// not) plus fixes that have no Lynis finding to match against.
type Report struct {
	Findings []Finding
	Native   []Fix
}

// Hidden returns the findings that are not actionable.
func (r Report) Hidden() []Finding {
	var hidden []Finding
	for _, f := range r.Findings {
		if !f.Actionable {
			hidden = append(hidden, f)
		}
	}
	return hidden
}

func hasSolution(solution string) bool {
	return solution != "" && solution != "-"
}

// Build merges Lynis findings with resolved themis fixes.
func Build(findings []lynis.Finding, fixes []Fix) Report {
	byLynisID := make(map[string][]Fix, len(fixes))
	for _, f := range fixes {
		byLynisID[f.LynisID] = append(byLynisID[f.LynisID], f)
	}

	report := Report{Findings: make([]Finding, 0, len(findings))}
	matched := make(map[string]bool, len(fixes))

	for _, f := range findings {
		tracked := byLynisID[f.TestID]
		for _, t := range tracked {
			matched[t.TestID] = true
		}
		report.Findings = append(report.Findings, Finding{
			TestID:      f.TestID,
			Kind:        f.Kind,
			Description: f.Description,
			Solution:    f.Solution,
			Fixes:       tracked,
			Actionable:  len(tracked) > 0 || hasSolution(f.Solution) || f.Kind == "warning",
		})
	}

	for _, f := range fixes {
		if !matched[f.TestID] {
			report.Native = append(report.Native, f)
		}
	}
	return report
}
