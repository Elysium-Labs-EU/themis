// Package state persists rollback metadata written by `themis apply` and
// consumed by `themis rollback`.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// DefaultPath is where apply/rollback state lives by default.
const DefaultPath = "/var/lib/themis/state.json"

// Entry is one applied Fix's revert data, keyed by its registry TestID.
type Entry struct {
	TestID     string `json:"test_id"`
	RevertData []byte `json:"revert_data"`
}

// Snapshot is everything `themis rollback` needs to undo one `apply` run.
type Snapshot struct {
	AppliedAt time.Time `json:"applied_at"`
	Entries   []Entry   `json:"entries"`
}

// Save writes snap to path, creating parent directories as needed. The
// write goes to a temp file in the same directory and is renamed into
// place: a rename replaces whatever directory entry currently sits at
// path — including a symlink planted there — without ever writing
// through it, and it is atomic, so a crash mid-write can't leave a
// truncated/partial state.json for a later Load to trust.
func Save(path string, snap Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	dir := filepath.Dir(path)
	if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
		return fmt.Errorf("creating state dir for %s: %w", path, mkdirErr)
	}

	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp state file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing state %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing state %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("writing state %s: %w", path, err)
	}
	return nil
}

// Load reads the snapshot written by the most recent Save. It first
// treats path as hostile: a locally-writable state directory would let
// another user plant a symlink or swap in their own state.json ahead of
// `themis rollback` running as root, whose entries.revert_data flows
// straight into root-privileged file writes and commands (see
// internal/fix). O_NOFOLLOW makes the open itself refuse a symlink at
// path, and the ownership/mode check runs against the opened file
// descriptor (not a second stat of the path) so there is no window
// between the check and the read for the file to be swapped.
func Load(path string) (Snapshot, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0) //nolint:gosec // path is the fixed state-file constant, not user input; O_NOFOLLOW rejects symlinks
	if err != nil {
		return Snapshot{}, fmt.Errorf("reading state %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return Snapshot{}, fmt.Errorf("reading state %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Snapshot{}, fmt.Errorf("state %s failed integrity check: not a regular file", path)
	}
	if verifyErr := verifyOwnerAndMode(info, os.Geteuid()); verifyErr != nil {
		return Snapshot{}, fmt.Errorf("state %s failed integrity check: %w", path, verifyErr)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return Snapshot{}, fmt.Errorf("reading state %s: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshaling state %s: %w", path, err)
	}
	return snap, nil
}

// verifyOwnerAndMode rejects a state file that isn't owned by wantUID or
// that grants group/other any access — either means someone besides the
// user running themis could have written or swapped it. Pure — info and
// wantUID are already-resolved inputs, no I/O here.
func verifyOwnerAndMode(info fs.FileInfo, wantUID int) error {
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("mode %04o is accessible to group/other, want 0600", perm)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot determine file owner")
	}
	if int(stat.Uid) != wantUID {
		return fmt.Errorf("owned by uid %d, want %d", stat.Uid, wantUID)
	}
	return nil
}

// Upsert returns a copy of entries with e appended, or with the existing
// entry sharing e.TestID replaced by e if one is already present. Pure —
// entries is never mutated in place, so a caller holding the original slice
// still sees the pre-upsert values.
func Upsert(entries []Entry, e Entry) []Entry {
	out := make([]Entry, 0, len(entries)+1)
	replaced := false
	for _, existing := range entries {
		if existing.TestID == e.TestID {
			out = append(out, e)
			replaced = true
			continue
		}
		out = append(out, existing)
	}
	if !replaced {
		out = append(out, e)
	}
	return out
}

// Without returns a copy of entries with the entry matching testID removed,
// if present. Pure — entries is never mutated in place.
func Without(entries []Entry, testID string) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.TestID == testID {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Clear removes the state file after a successful rollback. Missing file
// is not an error — there is nothing to clear.
func Clear(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing state %s: %w", path, err)
	}
	return nil
}
