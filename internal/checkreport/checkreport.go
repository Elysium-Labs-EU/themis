// Package checkreport merges audit source findings with themis's tracked
// fixes into one report, shared by the human (cmd/check.go) and machine
// (cmd/api_check.go) output paths.
package checkreport

import (
	"slices"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
)

// Fix is a themis-tracked fix, resolved against the source test ID it
// addresses.
type Fix struct {
	TestID      string
	LynisID     string
	Description string
	Satisfied   bool
}

// Finding is one audit finding merged with any themis fixes that track it.
// The same TestID+Kind reported by multiple sources collapses into a
// single Finding; Sources lists every source that reported it.
type Finding struct {
	TestID      string
	Kind        string
	Description string
	// Solution is a source's own remediation hint, e.g. a command to
	// run or setting to change. "-" when the source gave none.
	Solution string
	// Sources lists the name of every audit source that reported this
	// finding (e.g. "lynis").
	Sources []string
	Fixes   []Fix
	// Actionable is false when nothing points to a next step: no themis
	// fix tracks it, no source gave a solution hint, and it's not a
	// warning (the source's own higher-severity bucket) — safe to hide
	// by default.
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

// Build merges findings from one or more audit sources with resolved
// themis fixes. A finding sharing a TestID and Kind with one already seen
// (e.g. reported by two sources) is collapsed into the existing entry
// rather than duplicated.
func Build(findings []audit.Finding, fixes []Fix) Report {
	byLynisID := make(map[string][]Fix, len(fixes))
	for _, f := range fixes {
		byLynisID[f.LynisID] = append(byLynisID[f.LynisID], f)
	}

	report := Report{Findings: make([]Finding, 0, len(findings))}
	matched := make(map[string]bool, len(fixes))
	seen := make(map[string]int, len(findings))

	for _, f := range findings {
		key := f.TestID + "|" + f.Kind
		if idx, ok := seen[key]; ok {
			existing := &report.Findings[idx]
			if !slices.Contains(existing.Sources, f.Source) {
				existing.Sources = append(existing.Sources, f.Source)
			}
			continue
		}

		tracked := byLynisID[f.TestID]
		for _, t := range tracked {
			matched[t.TestID] = true
		}
		report.Findings = append(report.Findings, Finding{
			TestID:      f.TestID,
			Kind:        f.Kind,
			Description: f.Description,
			Solution:    f.Solution,
			Sources:     []string{f.Source},
			Fixes:       tracked,
			Actionable:  len(tracked) > 0 || hasSolution(f.Solution) || f.Kind == "warning",
		})
		seen[key] = len(report.Findings) - 1
	}

	for _, f := range fixes {
		if !matched[f.TestID] {
			report.Native = append(report.Native, f)
		}
	}
	return report
}
