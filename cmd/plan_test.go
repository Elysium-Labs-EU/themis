package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
)

func TestResolveFixesCoversWholeRegistry(t *testing.T) {
	planned, err := resolveFixes()
	if err != nil {
		t.Fatalf("resolveFixes: %v", err)
	}
	if len(planned) != len(fix.Registry) {
		t.Fatalf("got %d planned fixes, want %d", len(planned), len(fix.Registry))
	}
	for i := 1; i < len(planned); i++ {
		if planned[i-1].TestID >= planned[i].TestID {
			t.Fatalf("expected sorted TestIDs, got %q before %q", planned[i-1].TestID, planned[i].TestID)
		}
	}
}

func TestPlanCmdRunEPrintsSummary(t *testing.T) {
	buf := &bytes.Buffer{}
	planCmd.SetOut(buf)
	defer planCmd.SetOut(nil)

	if err := planCmd.RunE(planCmd, nil); err != nil {
		t.Fatalf("planCmd.RunE: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected plan output, got none")
	}
}

// TestResolveFixesMarksWarnedFixesInsteadOfApply is the regression test for
// issue #21: plan must not report [+apply] for a fix that apply would
// actually skip with [warn]. An unsatisfied fix whose Warn() fires should
// come back Warned, not counted as something that would be applied.
func TestResolveFixesMarksWarnedFixesInsteadOfApply(t *testing.T) {
	withRegistry(t, map[string]fix.Fix{
		"WARN-FIX": {
			TestID:      "WARN-FIX",
			Description: "a fix that warns instead of applying",
			Check:       func() (bool, error) { return false, nil },
			Apply:       func() ([]byte, error) { t.Fatal("Apply should not run for a warned fix"); return nil, nil },
			Revert:      func([]byte) error { return nil },
			Warn:        func() (string, bool, error) { return "detected a conflicting tool", true, nil },
		},
	})

	planned, err := resolveFixes()
	if err != nil {
		t.Fatalf("resolveFixes: %v", err)
	}
	if len(planned) != 1 {
		t.Fatalf("got %d planned fixes, want 1", len(planned))
	}
	p := planned[0]
	if p.Satisfied {
		t.Fatal("expected WARN-FIX to be unsatisfied")
	}
	if !p.Warned {
		t.Fatal("expected WARN-FIX to be Warned, matching apply's own Warn check")
	}
	if p.WarnMessage != "detected a conflicting tool" {
		t.Fatalf("WarnMessage = %q, want %q", p.WarnMessage, "detected a conflicting tool")
	}
}

// TestPlanCmdRunEShowsWarnNotApplyForWarnedFix pins the CLI-visible
// behavior: a warned fix's line must say [warn], never [+apply], and must
// not be counted toward "would be applied" in the summary.
func TestPlanCmdRunEShowsWarnNotApplyForWarnedFix(t *testing.T) {
	withRegistry(t, map[string]fix.Fix{
		"WARN-FIX": {
			TestID:      "WARN-FIX",
			Description: "a fix that warns instead of applying",
			Check:       func() (bool, error) { return false, nil },
			Apply:       func() ([]byte, error) { t.Fatal("Apply should not run for a warned fix"); return nil, nil },
			Revert:      func([]byte) error { return nil },
			Warn:        func() (string, bool, error) { return "detected a conflicting tool", true, nil },
		},
	})

	buf := &bytes.Buffer{}
	planCmd.SetOut(buf)
	defer planCmd.SetOut(nil)

	if err := planCmd.RunE(planCmd, nil); err != nil {
		t.Fatalf("planCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[warn]") || !strings.Contains(out, "WARN-FIX") {
		t.Fatalf("expected output to report WARN-FIX as [warn], got:\n%s", out)
	}
	if strings.Contains(out, "[+apply]") {
		t.Fatalf("expected no [+apply] line for a warned fix, got:\n%s", out)
	}
	if !strings.Contains(out, "0 fix(es) would be applied") {
		t.Fatalf("expected a warned fix to not count toward \"would be applied\", got:\n%s", out)
	}
	if !strings.Contains(out, "1 would be skipped with a warning") {
		t.Fatalf("expected summary to report 1 warned fix, got:\n%s", out)
	}
}
