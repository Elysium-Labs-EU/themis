package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/Elysium-Labs-EU/themis/internal/state"
)

// revertRecordingFix builds a fix whose Revert records the revert data it
// was called with, so a test can assert exactly which entries were reverted
// and which were left alone.
func revertRecordingFix(testID string, got *[]byte, err error) fix.Fix {
	return fix.Fix{
		TestID:      testID,
		Description: testID + " test fix",
		Check:       func() (bool, error) { return false, nil },
		Apply:       func() ([]byte, error) { return nil, nil },
		Revert: func(revertData []byte) error {
			*got = revertData
			return err
		},
	}
}

// driftWarningFix builds a fix whose RevertWarn always reports drift, and
// whose Revert records the revert data it was called with (so a test can
// assert Revert was or wasn't reached).
func driftWarningFix(testID, warnMsg string, got *[]byte) fix.Fix {
	return fix.Fix{
		TestID:      testID,
		Description: testID + " test fix",
		Check:       func() (bool, error) { return false, nil },
		Apply:       func() ([]byte, error) { return nil, nil },
		RevertWarn: func([]byte) (string, bool, error) {
			return warnMsg, true, nil
		},
		Revert: func(revertData []byte) error {
			*got = revertData
			return nil
		},
	}
}

func TestRollbackAllRevertsEveryEntryAndClearsState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{
			{TestID: "A-FIX", RevertData: []byte("a")},
			{TestID: "B-FIX", RevertData: []byte("b")},
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA, gotB []byte
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": revertRecordingFix("A-FIX", &gotA, nil),
		"B-FIX": revertRecordingFix("B-FIX", &gotB, nil),
	})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackAll(cmd, statePath, false); err != nil {
		t.Fatalf("runRollbackAll: %v", err)
	}
	if string(gotA) != "a" || string(gotB) != "b" {
		t.Fatalf("gotA=%q gotB=%q, want a and b reverted", gotA, gotB)
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state file to be cleared after full rollback")
	}
}

func TestRollbackOneRevertsOnlyThatEntryAndKeepsRest(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{
			{TestID: "A-FIX", RevertData: []byte("a")},
			{TestID: "B-FIX", RevertData: []byte("b")},
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA, gotB []byte
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": revertRecordingFix("A-FIX", &gotA, nil),
		"B-FIX": revertRecordingFix("B-FIX", &gotB, nil),
	})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX", false); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}
	if string(gotA) != "a" {
		t.Fatalf("gotA = %q, want %q (A-FIX should have been reverted)", gotA, "a")
	}
	if gotB != nil {
		t.Fatalf("gotB = %q, want nil (B-FIX must not be touched)", gotB)
	}

	snap, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load after single rollback: %v", err)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "B-FIX" {
		t.Fatalf("entries after single rollback = %+v, want [B-FIX]", snap.Entries)
	}
}

func TestRollbackOneClearsStateWhenLastEntryRemoved(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA []byte
	withRegistry(t, map[string]fix.Fix{"A-FIX": revertRecordingFix("A-FIX", &gotA, nil)})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX", false); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state file to be cleared once the last entry is removed")
	}
}

func TestRollbackOneUnknownTestIDErrorsWithoutMutatingState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA []byte
	withRegistry(t, map[string]fix.Fix{"A-FIX": revertRecordingFix("A-FIX", &gotA, nil)})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	err := runRollbackOne(cmd, statePath, "NOT-PRESENT", false)
	if err == nil {
		t.Fatal("expected an error for an unrecorded TestID")
	}
	if !strings.Contains(err.Error(), "NOT-PRESENT") {
		t.Errorf("error = %v, want it to mention NOT-PRESENT", err)
	}
	if gotA != nil {
		t.Fatal("A-FIX should not have been reverted when the requested TestID doesn't exist")
	}

	snap, loadErr := state.Load(statePath)
	if loadErr != nil {
		t.Fatalf("Load: %v", loadErr)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("entries after failed rollback = %+v, want unchanged [A-FIX]", snap.Entries)
	}
}

func TestRollbackOneStopsOnRevertError(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA []byte
	withRegistry(t, map[string]fix.Fix{"A-FIX": revertRecordingFix("A-FIX", &gotA, errors.New("boom"))})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX", false); err == nil {
		t.Fatal("expected runRollbackOne to propagate the Revert error")
	}

	snap, loadErr := state.Load(statePath)
	if loadErr != nil {
		t.Fatalf("Load: %v", loadErr)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("entries after failed revert = %+v, want unchanged [A-FIX] (state must not be rewritten on failure)", snap.Entries)
	}
}

func TestRollbackOneSkipsUnregisteredTestIDButStillRemovesEntry(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{
			{TestID: "GONE-FIX", RevertData: []byte("gone")},
			{TestID: "B-FIX", RevertData: []byte("b")},
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotB []byte
	withRegistry(t, map[string]fix.Fix{"B-FIX": revertRecordingFix("B-FIX", &gotB, nil)})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "GONE-FIX", false); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}

	snap, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "B-FIX" {
		t.Fatalf("entries = %+v, want [B-FIX]", snap.Entries)
	}
}

func TestRollbackOneSkipsOnDriftAndKeepsEntry(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA []byte
	withRegistry(t, map[string]fix.Fix{"A-FIX": driftWarningFix("A-FIX", "file changed since apply", &gotA)})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX", false); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}
	if gotA != nil {
		t.Fatal("Revert should not have run when RevertWarn detected drift and force was false")
	}
	if !strings.Contains(buf.String(), "file changed since apply") {
		t.Errorf("output = %q, want it to include the drift warning message", buf.String())
	}

	snap, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load after skipped rollback: %v", err)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("entries after skipped rollback = %+v, want unchanged [A-FIX] so a later --force can still revert it", snap.Entries)
	}
}

func TestRollbackOneForceRevertsDespiteDrift(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{{TestID: "A-FIX", RevertData: []byte("a")}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA []byte
	withRegistry(t, map[string]fix.Fix{"A-FIX": driftWarningFix("A-FIX", "file changed since apply", &gotA)})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackOne(cmd, statePath, "A-FIX", true); err != nil {
		t.Fatalf("runRollbackOne: %v", err)
	}
	if string(gotA) != "a" {
		t.Fatalf("gotA = %q, want %q (force should revert despite drift)", gotA, "a")
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected state file to be cleared after a forced revert")
	}
}

func TestRollbackAllSkipsDriftedEntriesButRevertsRest(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.Snapshot{
		Entries: []state.Entry{
			{TestID: "A-FIX", RevertData: []byte("a")},
			{TestID: "B-FIX", RevertData: []byte("b")},
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotA, gotB []byte
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": driftWarningFix("A-FIX", "file changed since apply", &gotA),
		"B-FIX": revertRecordingFix("B-FIX", &gotB, nil),
	})

	buf := &bytes.Buffer{}
	cmd := rollbackCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runRollbackAll(cmd, statePath, false); err != nil {
		t.Fatalf("runRollbackAll: %v", err)
	}
	if gotA != nil {
		t.Fatal("A-FIX should not have been reverted — it reported drift")
	}
	if string(gotB) != "b" {
		t.Fatalf("gotB = %q, want %q (B-FIX has no drift and should still be reverted)", gotB, "b")
	}

	snap, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load after partial rollback: %v", err)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("entries after partial rollback = %+v, want [A-FIX] retained for a later --force", snap.Entries)
	}
}
