package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := Snapshot{
		AppliedAt: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC),
		Entries: []Entry{
			{TestID: "SSH-7408-ROOTLOGIN", RevertData: []byte("PermitRootLogin yes\n")},
			{TestID: "KRNL-6000", RevertData: nil},
		},
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.AppliedAt.Equal(want.AppliedAt) {
		t.Errorf("AppliedAt = %v, want %v", got.AppliedAt, want.AppliedAt)
	}
	if len(got.Entries) != len(want.Entries) {
		t.Fatalf("Entries = %+v, want %+v", got.Entries, want.Entries)
	}
	for i, e := range got.Entries {
		if e.TestID != want.Entries[i].TestID {
			t.Errorf("Entries[%d].TestID = %q, want %q", i, e.TestID, want.Entries[i].TestID)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error loading missing file")
	}
}

func TestClearMissingFileIsNotError(t *testing.T) {
	if err := Clear(filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatalf("Clear on missing file: %v", err)
	}
}

func TestClearRemovesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, Snapshot{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Clear(path); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to fail after Clear")
	}
}
