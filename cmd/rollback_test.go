package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
)

func revertibleFix(testID string, reverted *bool) fix.Fix {
	return fix.Fix{
		TestID:      testID,
		Description: "test fix",
		Check:       func() (bool, error) { return false, nil },
		Apply:       func() ([]byte, error) { return []byte("data"), nil },
		Revert:      func([]byte) error { *reverted = true; return nil },
	}
}

func seedState(t *testing.T, statePath string, entries ...state.Entry) {
	t.Helper()
	if err := state.Save(statePath, state.Snapshot{Entries: entries}); err != nil {
		t.Fatalf("seeding state: %v", err)
	}
}

// TestRollbackSkipsDriftedFixAndPreservesState is the regression test for
// issue #16: when a Fix reports RevertWarn drift, rollback must not call
// Revert (which would silently discard the operator's post-apply edit),
// and must keep the entry in state.json so a later `rollback --force` can
// still undo it.
func TestRollbackSkipsDriftedFixAndPreservesState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	reverted := false
	f := revertibleFix("DRIFT-FIX", &reverted)
	f.RevertWarn = func([]byte) (string, bool, error) {
		return "DRIFT-FIX's config changed since apply", true, nil
	}
	withRegistry(t, map[string]fix.Fix{"DRIFT-FIX": f})
	seedState(t, statePath, state.Entry{TestID: "DRIFT-FIX", RevertData: []byte("data")})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollback(cmd, statePath, false); err != nil {
		t.Fatalf("runRollback: %v", err)
	}
	if reverted {
		t.Fatal("expected Revert not to be called when RevertWarn detects drift")
	}
	if !strings.Contains(buf.String(), "config changed since apply") {
		t.Fatalf("expected warning message in output, got %q", buf.String())
	}

	snap, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("expected state to be preserved for the skipped fix: %v", err)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "DRIFT-FIX" {
		t.Fatalf("state entries = %+v, want [DRIFT-FIX] preserved for a future --force rollback", snap.Entries)
	}
}

// TestRollbackForceRevertsDriftedFix confirms --force overrides the
// RevertWarn skip and reverts anyway, clearing state once nothing remains.
func TestRollbackForceRevertsDriftedFix(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	reverted := false
	f := revertibleFix("DRIFT-FIX", &reverted)
	f.RevertWarn = func([]byte) (string, bool, error) {
		return "DRIFT-FIX's config changed since apply", true, nil
	}
	withRegistry(t, map[string]fix.Fix{"DRIFT-FIX": f})
	seedState(t, statePath, state.Entry{TestID: "DRIFT-FIX", RevertData: []byte("data")})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollback(cmd, statePath, true); err != nil {
		t.Fatalf("runRollback: %v", err)
	}
	if !reverted {
		t.Fatal("expected --force to revert despite detected drift")
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state to be cleared once every entry is reverted")
	}
}

// TestRollbackNoWarnRevertsNormally ensures a Fix with no RevertWarn (or
// one that reports no drift) still reverts exactly as before.
func TestRollbackNoWarnRevertsNormally(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	reverted := false
	withRegistry(t, map[string]fix.Fix{"PLAIN-FIX": revertibleFix("PLAIN-FIX", &reverted)})
	seedState(t, statePath, state.Entry{TestID: "PLAIN-FIX", RevertData: []byte("data")})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollback(cmd, statePath, false); err != nil {
		t.Fatalf("runRollback: %v", err)
	}
	if !reverted {
		t.Fatal("expected Revert to be called when there is no drift to warn about")
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state to be cleared once every entry is reverted")
	}
}

// TestRollbackPreservesOrderOfMultipleSkippedFixes ensures skipped entries
// are re-saved in original (apply) order, so a subsequent --force rollback
// still unwinds them LIFO rather than in whatever order they happened to be
// skipped.
func TestRollbackPreservesOrderOfMultipleSkippedFixes(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	revertedA, revertedB := false, false
	fA := revertibleFix("A-FIX", &revertedA)
	fA.RevertWarn = func([]byte) (string, bool, error) { return "A drifted", true, nil }
	fB := revertibleFix("B-FIX", &revertedB)
	fB.RevertWarn = func([]byte) (string, bool, error) { return "B drifted", true, nil }
	withRegistry(t, map[string]fix.Fix{"A-FIX": fA, "B-FIX": fB})
	seedState(t, statePath,
		state.Entry{TestID: "A-FIX", RevertData: []byte("a")},
		state.Entry{TestID: "B-FIX", RevertData: []byte("b")},
	)

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollback(cmd, statePath, false); err != nil {
		t.Fatalf("runRollback: %v", err)
	}
	if revertedA || revertedB {
		t.Fatal("expected neither fix to be reverted")
	}

	snap, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snap.Entries) != 2 || snap.Entries[0].TestID != "A-FIX" || snap.Entries[1].TestID != "B-FIX" {
		t.Fatalf("state entries = %+v, want [A-FIX B-FIX] in original order", snap.Entries)
	}
}
