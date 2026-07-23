package osquery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
	"github.com/Elysium-Labs-EU/themis/internal/state"
)

// queryFunc runs an osquery SQL query and returns its rows. A field
// (rather than always calling Query directly) so tests can drive Source
// without a real osqueryi binary.
type queryFunc func(ctx context.Context, sql string) ([]Row, error)

// Source detects drift: themis fixes that a prior `themis apply` run
// confirmed satisfied (per the persisted state.Snapshot) but that no
// longer hold, re-verified independently via osquery. It plugs into
// audit.Run alongside lynis and themis-native, but — unlike them — only
// ever reports on fixes this host has actually applied before; a fix
// that was never applied has nothing to drift from.
//
// Both a missing apply-state file and a missing osqueryi binary are
// treated as "nothing to report" rather than an error: osquery is an
// optional dependency only needed for drift detection, and most hosts
// won't have run `themis apply` (or installed osquery) yet.
type Source struct {
	query     queryFunc
	statePath string
}

// NewSource returns an osquery drift-detection audit.Source. statePath
// overrides state.DefaultPath — pass "" in production to use the
// default.
func NewSource(statePath string) Source {
	return Source{statePath: statePath, query: Query}
}

// newSourceWith builds a Source with an injected query func, for tests.
func newSourceWith(statePath string, query queryFunc) Source {
	return Source{statePath: statePath, query: query}
}

// Name identifies this source as "osquery".
func (Source) Name() string { return "osquery" }

// Run loads the last apply snapshot, re-verifies via osquery every
// applied fix this package has a DriftCheck for, and returns a
// drift-kind Finding for each that no longer holds.
func (s Source) Run(ctx context.Context) ([]audit.Finding, error) {
	path := s.statePath
	if path == "" {
		path = state.DefaultPath
	}

	snap, err := loadSnapshot(path)
	if err != nil {
		return nil, err
	}
	if len(snap.Entries) == 0 {
		return nil, nil // apply has never run — nothing to have drifted from
	}

	applied := make(map[string]time.Time, len(snap.Entries))
	for _, e := range snap.Entries {
		applied[e.TestID] = snap.AppliedAt
	}

	var findings []audit.Finding
	for _, c := range Checks {
		appliedAt, wasApplied := applied[c.TestID]
		if !wasApplied {
			continue
		}
		rows, err := s.query(ctx, c.Query)
		if err != nil {
			if errors.Is(err, ErrNotInstalled) {
				return nil, nil // osquery not installed — drift detection is opt-in
			}
			return nil, fmt.Errorf("checking %s for drift: %w", c.TestID, err)
		}
		if c.Satisfied(rows) {
			continue
		}
		findings = append(findings, driftFinding(c, appliedAt))
	}
	return findings, nil
}

// loadSnapshot returns the state.Snapshot at path, or a zero Snapshot
// (no error) if apply has never run.
func loadSnapshot(path string) (state.Snapshot, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return state.Snapshot{}, nil
	}
	snap, err := state.Load(path)
	if err != nil {
		return state.Snapshot{}, fmt.Errorf("loading apply state: %w", err)
	}
	return snap, nil
}

// driftFinding builds the audit.Finding reported when c no longer holds.
// Pure — no I/O.
func driftFinding(c DriftCheck, appliedAt time.Time) audit.Finding {
	return audit.Finding{
		TestID:      c.TestID,
		Description: c.Description,
		Details:     fmt.Sprintf("applied %s, no longer satisfied", appliedAt.Format(time.RFC3339)),
		Solution:    "-",
		Kind:        "drift",
		Source:      "osquery",
	}
}
