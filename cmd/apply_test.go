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

// TestApplyRecordsPartialRevertDataOnError is the regression test for
// issue #10: a Fix.Apply() that writes its target file and then fails at a
// later step (e.g. a service reload) may return non-nil revertData
// alongside the error. That revert data must still be recorded in
// state.json — otherwise the write is real but themis has no way to know
// about it, and rollback can never undo it.
func TestApplyRecordsPartialRevertDataOnError(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "first fix", func() ([]byte, error) {
			return []byte("a"), nil
		}),
		"B-FIX": unsatisfiedFix("B-FIX", "second fix, fails after a partial write", func() ([]byte, error) {
			return []byte("partial-b"), errors.New("post-write reload failed")
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
		t.Fatalf("Load after partial failure: %v (partial revert data for B-FIX should already be on disk)", loadErr)
	}
	if len(snap.Entries) != 2 {
		t.Fatalf("state after partial failure = %+v, want 2 entries (A-FIX, B-FIX with partial revert data)", snap.Entries)
	}
	if snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("snap.Entries[0].TestID = %q, want A-FIX", snap.Entries[0].TestID)
	}
	bEntry := snap.Entries[1]
	if bEntry.TestID != "B-FIX" {
		t.Fatalf("snap.Entries[1].TestID = %q, want B-FIX", bEntry.TestID)
	}
	if string(bEntry.RevertData) != "partial-b" {
		t.Fatalf("B-FIX RevertData = %q, want %q", bEntry.RevertData, "partial-b")
	}
}

// TestApplyDoesNotRecordEntryWhenApplyFailsCleanly ensures the pre-existing
// behavior is unchanged for a Fix.Apply() that fails without returning any
// revert data: nothing new happened, so nothing new should be recorded.
func TestApplyDoesNotRecordEntryWhenApplyFailsCleanly(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "fails with no revert data", func() ([]byte, error) {
			return nil, errors.New("boom, nothing written")
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

	if _, loadErr := state.Load(statePath); loadErr == nil {
		t.Fatal("expected no state file to be written when Apply failed with no revert data")
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
