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
