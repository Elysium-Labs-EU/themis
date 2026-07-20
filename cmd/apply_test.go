package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
)

// withRegistry temporarily swaps fix.Registry so apply's mutating Apply()
// calls never touch the real host, restoring the original on cleanup.
func withRegistry(t *testing.T, reg map[string]fix.Fix) {
	t.Helper()
	orig := fix.Registry
	fix.Registry = reg
	t.Cleanup(func() { fix.Registry = orig })
}

func unsatisfiedFix(testID, description string, apply func() ([]byte, error)) fix.Fix {
	return fix.Fix{
		TestID:      testID,
		Description: description,
		Check:       func() (bool, error) { return false, nil },
		Apply:       apply,
		Revert:      func([]byte) error { return nil },
	}
}

// TestApplySavesStateAfterEveryFix is the regression test for issue #19:
// state.Save must be called after each successful Apply, not only once at
// the end of the loop, so a kill -9/SIGINT between two fixes can't lose
// rollback state for fixes that already succeeded.
func TestApplySavesStateAfterEveryFix(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	var sawEntriesBeforeB []string
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "first fix", func() ([]byte, error) {
			return []byte("a"), nil
		}),
		"B-FIX": unsatisfiedFix("B-FIX", "second fix", func() ([]byte, error) {
			// resolveFixes/runApply process TestIDs in sorted order, so
			// A-FIX has already run by the time B-FIX's Apply executes.
			// If state was only saved at the end of the loop, this Load
			// would fail (no file yet) or return zero entries. Load
			// succeeding with A-FIX already present proves the save
			// happened incrementally, mid-loop.
			snap, err := state.Load(statePath)
			if err != nil {
				return nil, err
			}
			for _, e := range snap.Entries {
				sawEntriesBeforeB = append(sawEntriesBeforeB, e.TestID)
			}
			return []byte("b"), nil
		}),
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}

	if len(sawEntriesBeforeB) != 1 || sawEntriesBeforeB[0] != "A-FIX" {
		t.Fatalf("state visible to B-FIX's Apply = %v, want [A-FIX] (incremental save didn't happen before B-FIX ran)", sawEntriesBeforeB)
	}

	final, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load final state: %v", err)
	}
	if len(final.Entries) != 2 || final.Entries[0].TestID != "A-FIX" || final.Entries[1].TestID != "B-FIX" {
		t.Fatalf("final state entries = %+v, want [A-FIX B-FIX]", final.Entries)
	}
}

// TestApplyPreservesStateForFixesAppliedBeforeAFailure simulates the
// kill -9 scenario indirectly: a fix fails partway through the run, and
// the state for everything that succeeded before it must already be on
// disk (not just referenced in the error message).
func TestApplyPreservesStateForFixesAppliedBeforeAFailure(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "first fix", func() ([]byte, error) {
			return []byte("a"), nil
		}),
		"B-FIX": unsatisfiedFix("B-FIX", "second fix", func() ([]byte, error) {
			return nil, errors.New("boom")
		}),
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	err := runApply(cmd, statePath)
	if err == nil {
		t.Fatal("expected runApply to return an error when a fix fails")
	}

	snap, loadErr := state.Load(statePath)
	if loadErr != nil {
		t.Fatalf("Load after partial failure: %v (state for A-FIX should already be on disk)", loadErr)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("state after partial failure = %+v, want just A-FIX", snap.Entries)
	}
}

func TestApplyNothingToApplySkipsSave(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": {
			TestID:      "A-FIX",
			Description: "already satisfied",
			Check:       func() (bool, error) { return true, nil },
			Apply:       func() ([]byte, error) { t.Fatal("Apply should not run for a satisfied fix"); return nil, nil },
			Revert:      func([]byte) error { return nil },
		},
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected no state file to be written when nothing was applied")
	}
}
