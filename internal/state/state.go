// Package state persists rollback metadata written by `themis apply` and
// consumed by `themis rollback`.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Save writes snap to path, creating parent directories as needed.
func Save(path string, snap Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating state dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing state %s: %w", path, err)
	}
	return nil
}

// Load reads the snapshot written by the most recent Save.
func Load(path string) (Snapshot, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the fixed state-file constant, not user input
	if err != nil {
		return Snapshot{}, fmt.Errorf("reading state %s: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshaling state %s: %w", path, err)
	}
	return snap, nil
}

// Clear removes the state file after a successful rollback. Missing file
// is not an error — there is nothing to clear.
func Clear(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing state %s: %w", path, err)
	}
	return nil
}
