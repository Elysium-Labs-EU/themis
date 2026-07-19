package cmd

import (
	"bytes"
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
