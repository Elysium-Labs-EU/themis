package state

import (
	"os"
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

func TestLoadRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.json")
	if err := Save(target, Snapshot{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	link := filepath.Join(dir, "state.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if _, err := Load(link); err == nil {
		t.Fatal("expected Load to reject a symlinked state file")
	}
}

func TestLoadRejectsGroupOrOtherAccessibleMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, Snapshot{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to reject a group-readable state file")
	}
}

func TestUpsertAppendsNewTestID(t *testing.T) {
	entries := []Entry{{TestID: "A-FIX", RevertData: []byte("a")}}
	got := Upsert(entries, Entry{TestID: "B-FIX", RevertData: []byte("b")})

	if len(got) != 2 || got[0].TestID != "A-FIX" || string(got[0].RevertData) != "a" ||
		got[1].TestID != "B-FIX" || string(got[1].RevertData) != "b" {
		t.Fatalf("Upsert (new) = %+v, want [A-FIX:a B-FIX:b]", got)
	}
	if len(entries) != 1 {
		t.Fatalf("Upsert mutated its input slice: %+v", entries)
	}
}

func TestUpsertReplacesExistingTestID(t *testing.T) {
	entries := []Entry{
		{TestID: "A-FIX", RevertData: []byte("old")},
		{TestID: "B-FIX", RevertData: []byte("b")},
	}
	got := Upsert(entries, Entry{TestID: "A-FIX", RevertData: []byte("new")})

	if len(got) != 2 || got[0].TestID != "A-FIX" || string(got[0].RevertData) != "new" ||
		got[1].TestID != "B-FIX" || string(got[1].RevertData) != "b" {
		t.Fatalf("Upsert (replace) = %+v, want [A-FIX:new B-FIX:b]", got)
	}
	if string(entries[0].RevertData) != "old" {
		t.Fatalf("Upsert mutated its input slice: %+v", entries)
	}
}

func TestWithoutRemovesMatchingEntry(t *testing.T) {
	entries := []Entry{
		{TestID: "A-FIX", RevertData: []byte("a")},
		{TestID: "B-FIX", RevertData: []byte("b")},
	}
	got := Without(entries, "A-FIX")

	if len(got) != 1 || got[0].TestID != "B-FIX" {
		t.Fatalf("Without = %+v, want just B-FIX", got)
	}
	if len(entries) != 2 {
		t.Fatalf("Without mutated its input slice: %+v", entries)
	}
}

func TestWithoutOnMissingTestIDIsNoOp(t *testing.T) {
	entries := []Entry{{TestID: "A-FIX", RevertData: []byte("a")}}
	got := Without(entries, "NOT-THERE")

	if len(got) != 1 || got[0].TestID != "A-FIX" {
		t.Fatalf("Without (missing) = %+v, want unchanged [A-FIX]", got)
	}
}

func TestVerifyOwnerAndModeRejectsWrongUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, Snapshot{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if err := verifyOwnerAndMode(info, os.Geteuid()+1); err == nil {
		t.Fatal("expected verifyOwnerAndMode to reject a UID mismatch")
	}
	if err := verifyOwnerAndMode(info, os.Geteuid()); err != nil {
		t.Errorf("verifyOwnerAndMode with the real UID: %v", err)
	}
}
