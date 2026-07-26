// Package checkreport merges audit source findings with themis's tracked
// fixes into one report, shared by the human (cmd/check.go) and machine
// (cmd/api_check.go) output paths.
package checkreport

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
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
	// Details carries a drift finding's own context (e.g. when the fix
	// was applied and confirmed satisfied). Empty for a normal finding.
	Details string
	// Sources lists the name of every audit source that reported this
	// finding (e.g. "lynis").
	Sources []string
	Fixes   []Fix
	// Actionable is false when nothing points to a next step: no themis
	// fix tracks it, no source gave a solution hint, and it's not a
	// warning (the source's own higher-severity bucket) — safe to hide
	// by default.
	Actionable bool
	// Malformed is true when the source finding's own fields (TestID
	// shape, Kind, or field length) didn't pass validation. A malformed
	// finding is never Actionable and never matched against tracked
	// Fixes, regardless of what its Kind/Solution claim — the audit
	// source is external (e.g. lynis) and its output isn't trusted to
	// drive a pass/fail verdict on its own say-so.
	Malformed bool
}

// Report is the full merge: every audit finding (tagged actionable or
// not) plus fixes that have no finding, from any source, to match
// against.
type Report struct {
	Findings []Finding
	// Unmatched holds themis fixes whose tracked test ID was not
	// reported by any audit source this run — not "themis-native
	// fixes": a Lynis-tracked fix lands here just as readily when
	// Lynis's own scan happened to find nothing wrong with it.
	Unmatched []Fix
	// Drift holds "was satisfied, now isn't" findings from drift-capable
	// sources (currently internal/osquery's audit.Finding{Kind: "drift"}
	// results). Kept separate from Findings so a regression in a fix
	// that already ran doesn't read like just another never-applied
	// suggestion.
	Drift []Finding
}

// Hidden returns the findings that are not actionable.
func (r Report) Hidden() []Finding {
	var hidden []Finding
	for i := range r.Findings {
		if !r.Findings[i].Actionable {
			hidden = append(hidden, r.Findings[i])
		}
	}
	return hidden
}

func hasSolution(solution string) bool {
	return solution != "" && solution != "-"
}

// maxFieldLen bounds free-text finding fields (Description, Solution)
// from an audit source. Generous enough for any real Lynis line, small
// enough to stop a hostile source flooding the report or memory.
const maxFieldLen = 2000

// testIDPattern allowlists the Lynis test ID shape (e.g. "SSH-7408" or
// "HRDN-7230"): uppercase letters/digits, a hyphen, then digits. This
// also guards the "|"-joined dedup key in Build below — an unconstrained
// TestID containing "|" could otherwise be crafted to collide with an
// unrelated finding's key.
var testIDPattern = regexp.MustCompile(`^[A-Z0-9]{2,10}-[0-9]{3,6}$`)

// nativeTestIDPattern allowlists the shape of a themis-native TestID (e.g.
// "THEMIS-FAIL2BAN", "THEMIS-UNATTENDED-UPGRADES", registered in
// internal/fix/registry.go and reported by internal/native): a "THEMIS-"
// prefix followed by uppercase letters/digits/hyphens, length-bounded and
// "|"-free for the same dedup-key-spoofing reason as testIDPattern.
var nativeTestIDPattern = regexp.MustCompile(`^THEMIS-[A-Z0-9-]{1,40}$`)

// openscapTestIDPattern allowlists the shape of an OpenSCAP XCCDF rule id
// (e.g. "xccdf_org.ssgproject.content_rule_sshd_disable_root_login",
// reported by internal/openscap with Source "openscap"): an "xccdf_"
// prefix followed by lowercase letters/digits/underscores/dots. These ids
// are longer than Lynis/native ones, so the bound is looser, but it stays
// length-bounded and "|"-free for the same dedup-key-spoofing reason as
// the other two patterns.
var openscapTestIDPattern = regexp.MustCompile(`^xccdf_[a-z0-9_.]{1,200}$`)

// validKinds allowlists the two kinds a real audit source reports.
// Kind is one of the fields that decides Actionable, so an unexpected
// value can't be trusted to mean "warning".
var validKinds = map[string]bool{"suggestion": true, "warning": true}

// trustworthy reports whether a finding's TestID, Kind, description, and
// solution are shaped like a real audit finding: an allowlisted TestID for
// its claimed Source, allowlisted Kind, and free-text fields within a sane
// size. The TestID shape is source-specific — Source="themis" (this
// codebase's own native checks) validates against nativeTestIDPattern and
// Source="openscap" against openscapTestIDPattern, since each source's ids
// look nothing like Lynis's "SSH-7408" shape and would otherwise all fail
// testIDPattern and get wrongly flagged Malformed. Takes only the fields
// it needs rather than a whole audit.Finding. Pure — no I/O.
func trustworthy(source, testID, kind, description, solution string) bool {
	idPattern := testIDPattern
	switch source {
	case "themis":
		idPattern = nativeTestIDPattern
	case "openscap":
		idPattern = openscapTestIDPattern
	}
	return idPattern.MatchString(testID) &&
		validKinds[kind] &&
		len(description) <= maxFieldLen &&
		len(solution) <= maxFieldLen
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

	for i, f := range findings {
		// Drift findings come from an internal, trusted source
		// (internal/osquery), not the external audit sources the
		// trustworthiness check guards against, and carry their own
		// shape — route them out before validation.
		if f.Kind == "drift" {
			report.Drift = append(report.Drift, Finding{
				TestID:      f.TestID,
				Kind:        f.Kind,
				Description: f.Description,
				Details:     f.Details,
				Sources:     []string{f.Source},
				Actionable:  true,
			})
			continue
		}

		trusted := trustworthy(f.Source, f.TestID, f.Kind, f.Description, f.Solution)

		key := f.TestID + "|" + f.Kind
		if !trusted {
			// An untrustworthy TestID/Kind can't be used as a dedup key
			// either (e.g. an embedded "|" could collide with an
			// unrelated finding's key), so give it one that can never
			// match anything else.
			key = fmt.Sprintf("malformed#%d", i)
		}
		if idx, ok := seen[key]; ok {
			existing := &report.Findings[idx]
			if !slices.Contains(existing.Sources, f.Source) {
				existing.Sources = append(existing.Sources, f.Source)
			}
			continue
		}

		var tracked []Fix
		if trusted {
			tracked = byLynisID[f.TestID]
			for _, t := range tracked {
				matched[t.TestID] = true
			}
		}
		report.Findings = append(report.Findings, Finding{
			TestID:      truncate(f.TestID),
			Kind:        truncate(f.Kind),
			Description: truncate(f.Description),
			Solution:    truncate(f.Solution),
			Sources:     []string{f.Source},
			Fixes:       tracked,
			Malformed:   !trusted,
			Actionable:  trusted && (len(tracked) > 0 || hasSolution(f.Solution) || f.Kind == "warning"),
		})
		seen[key] = len(report.Findings) - 1
	}

	for _, f := range fixes {
		if !matched[f.TestID] {
			report.Unmatched = append(report.Unmatched, f)
		}
	}
	return report
}

// truncate caps s at maxFieldLen runes, appending a marker when it does.
// Pure — no I/O.
func truncate(s string) string {
	if len(s) <= maxFieldLen {
		return s
	}
	r := []rune(s)
	if len(r) <= maxFieldLen {
		return s
	}
	return string(r[:maxFieldLen]) + "…(truncated)"
}
