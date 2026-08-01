package ui

import (
	"regexp"
	"testing"
)

// stripANSI strips lipgloss/termenv color escape codes so width assertions
// operate on the visible text, not the ANSI-wrapped string.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// TestFixLabelWidthAligned guards the bug in issue #73: plan, apply, and
// rollback each padded their bracketed labels to different widths, so
// columns didn't line up across commands run back to back in one session.
// Every label FixLabel renders must come out the same visible width.
func TestFixLabelWidthAligned(t *testing.T) {
	words := []struct {
		word  string
		state FixStatus
	}{
		{"[ok]", StatusSatisfied},
		{"[warn]", StatusWarned},
		{"[+apply]", StatusPending},
		{"[applied]", StatusChanged},
		{"[failed]", StatusFailed},
		{"[skip]", StatusWarned},
		{"[reverted]", StatusChanged},
	}
	for _, w := range words {
		got := len(stripANSI(FixLabel(w.word, w.state)))
		if got != fixLabelWidth {
			t.Errorf("FixLabel(%q) visible width = %d, want %d", w.word, got, fixLabelWidth)
		}
	}
}

// TestFixLabelStyleByState checks that each semantic state renders with a
// distinct, deterministic style so the same state always looks the same no
// matter which command reports it.
func TestFixLabelStyleByState(t *testing.T) {
	successRendered := FixLabel("[ok]", StatusSatisfied)
	warnRendered := FixLabel("[warn]", StatusWarned)
	failRendered := FixLabel("[failed]", StatusFailed)
	if successRendered == warnRendered || successRendered == failRendered || warnRendered == failRendered {
		t.Errorf("expected distinct styles per state, got equal rendered output: ok=%q warn=%q failed=%q", successRendered, warnRendered, failRendered)
	}
}

// TestFixIconRendersWordUnpadded checks that, unlike FixLabel, FixIcon
// renders its word without padding to fixLabelWidth — it's meant for
// inline use (prose, a table cell), where fixed padding would just add
// stray spaces.
func TestFixIconRendersWordUnpadded(t *testing.T) {
	got := stripANSI(FixIcon("✓ fixed", StatusSatisfied))
	if got != "✓ fixed" {
		t.Errorf("FixIcon visible text = %q, want %q", got, "✓ fixed")
	}
}

// TestFixIconPreservesWordAcrossStates checks that FixIcon renders the
// same input word verbatim (modulo styling) regardless of state — the
// state only picks the color, matching how check.go calls it with
// different words per state ("✓ fixed" vs "○ apply").
func TestFixIconPreservesWordAcrossStates(t *testing.T) {
	for _, state := range []FixStatus{StatusSatisfied, StatusPending, StatusChanged, StatusWarned, StatusFailed} {
		got := stripANSI(FixIcon("○ apply", state))
		if got != "○ apply" {
			t.Errorf("FixIcon state=%v visible text = %q, want %q", state, got, "○ apply")
		}
	}
}
