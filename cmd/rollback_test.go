package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
)

func revertRecordingFix(testID string, reverted *[]string) fix.Fix {
	return fix.Fix{
		TestID:      testID,
		Description: testID,
		Check:       func() (bool, error) { return false, nil },
		Apply:       func() ([]byte, error) { return []byte(testID), nil },
		Revert: func([]byte) error {
			*reverted = append(*reverted, testID)
			return nil
		},
	}
}

// TestRollbackOneRevertsOnlyThatEntry is the regression test for issue #21:
// `themis rollback <TEST-ID>` must revert just the named fix and rewrite
// state.json with the remaining entries, instead of the bare `rollback`
// behavior of reverting and clearing everything.
func TestRollbackOneRevertsOnlyThatEntry(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	var reverted []string
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": revertRecordingFix("A-FIX", &reverted),
		"B-FIX": revertRecordingFix("B-FIX", &reverted),
	})

	snap := state.Snapshot{
		AppliedAt: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		Entries: []state.Entry{
			{TestID: "A-FIX", RevertData: []byte("a")},
			{TestID: "B-FIX", RevertData: []byte("b")},
		},
	}
	if err := state.Save(statePath, snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX"); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}

	if len(reverted) != 1 || reverted[0] != "A-FIX" {
		t.Fatalf("reverted = %v, want [A-FIX] (B-FIX must be left alone)", reverted)
	}

	got, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load after rollback: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].TestID != "B-FIX" {
		t.Fatalf("remaining state = %+v, want just B-FIX", got.Entries)
	}
}

// TestRollbackOneClearsFileWhenLastEntryRemoved ensures the state file is
// removed entirely once the last remaining entry is rolled back, matching
// the bare `rollback` command's end state.
func TestRollbackOneClearsFileWhenLastEntryRemoved(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	var reverted []string
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": revertRecordingFix("A-FIX", &reverted),
	})

	snap := state.Snapshot{
		AppliedAt: time.Now().UTC(),
		Entries:   []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}
	if err := state.Save(statePath, snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX"); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}

	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state file to be cleared once the last entry was rolled back")
	}
}

// TestRollbackOneUnknownTestIDErrors ensures rollback of a TestID with no
// recorded state fails rather than silently no-op'ing.
func TestRollbackOneUnknownTestIDErrors(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	snap := state.Snapshot{
		AppliedAt: time.Now().UTC(),
		Entries:   []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}
	if err := state.Save(statePath, snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "NOT-RECORDED"); err == nil {
		t.Fatal("expected an error for a TestID with no recorded rollback state")
	}

	got, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load after failed rollback: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].TestID != "A-FIX" {
		t.Fatalf("state after failed rollback = %+v, want unchanged [A-FIX]", got.Entries)
	}
}

// TestRollbackAllRevertsEverythingAndClears preserves the pre-existing bare
// `rollback` behavior: revert every entry LIFO, then clear the state file.
func TestRollbackAllRevertsEverythingAndClears(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	var reverted []string
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": revertRecordingFix("A-FIX", &reverted),
		"B-FIX": revertRecordingFix("B-FIX", &reverted),
	})

	snap := state.Snapshot{
		AppliedAt: time.Now().UTC(),
		Entries: []state.Entry{
			{TestID: "A-FIX", RevertData: []byte("a")},
			{TestID: "B-FIX", RevertData: []byte("b")},
		},
	}
	if err := state.Save(statePath, snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackAll(cmd, statePath); err != nil {
		t.Fatalf("runRollbackAll: %v", err)
	}

	if len(reverted) != 2 || reverted[0] != "B-FIX" || reverted[1] != "A-FIX" {
		t.Fatalf("reverted = %v, want [B-FIX A-FIX] (LIFO order)", reverted)
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state file to be cleared after rollback of all entries")
	}
}
