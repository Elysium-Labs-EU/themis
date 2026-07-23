package osquery

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elysium-Labs-EU/themis/internal/state"
)

func TestSourceNameIsOsquery(t *testing.T) {
	if got := NewSource("").Name(); got != "osquery" {
		t.Errorf("Name() = %q, want %q", got, "osquery")
	}
}

func TestRunReturnsNoFindingsWhenApplyHasNeverRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := newSourceWith(path, func(context.Context, string) ([]Row, error) {
		t.Fatal("query should never run when there is no apply state")
		return nil, nil
	})

	findings, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestRunReturnsNoFindingsWhenAppliedFixesStillHold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	seedSnapshot(t, path)

	s := newSourceWith(path, func(_ context.Context, sql string) ([]Row, error) {
		return []Row{{"active_state": "active"}}, nil
	})

	findings, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestRunReportsDriftForAnAppliedFixThatNoLongerHolds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	seedSnapshot(t, path)

	s := newSourceWith(path, func(_ context.Context, sql string) ([]Row, error) {
		return []Row{{"active_state": "inactive"}}, nil
	})

	findings, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 drift finding, got %+v", findings)
	}
	f := findings[0]
	if f.TestID != "THEMIS-FAIL2BAN" || f.Kind != "drift" || f.Source != "osquery" {
		t.Errorf("finding = %+v, want TestID=THEMIS-FAIL2BAN Kind=drift Source=osquery", f)
	}
}

func TestRunSkipsChecksForFixesNeverApplied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	seedSnapshot(t, path) // KRNL-6000, SSH-* etc. were never applied

	queried := 0
	s := newSourceWith(path, func(_ context.Context, sql string) ([]Row, error) {
		queried++
		return []Row{{"active_state": "active"}}, nil
	})

	if _, err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if queried != 1 {
		t.Fatalf("expected exactly 1 query (for the one applied fix), got %d", queried)
	}
}

func TestRunPropagatesQueryErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	seedSnapshot(t, path)

	s := newSourceWith(path, func(context.Context, string) ([]Row, error) {
		return nil, errBoom
	})

	if _, err := s.Run(context.Background()); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestRunReturnsNoFindingsWhenOsqueryNotInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	seedSnapshot(t, path)

	s := newSourceWith(path, func(context.Context, string) ([]Row, error) {
		return nil, fmt.Errorf("running osqueryi: %w", ErrNotInstalled)
	})

	findings, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings when osqueryi is missing, got %+v", findings)
	}
}

var errBoom = errors.New("boom")

// seedSnapshot writes a state.Snapshot to path recording THEMIS-FAIL2BAN
// as applied, for tests that need a prior `themis apply` run to have
// happened.
func seedSnapshot(t *testing.T, path string) {
	t.Helper()
	snap := state.Snapshot{
		AppliedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Entries:   []state.Entry{{TestID: "THEMIS-FAIL2BAN"}},
	}
	if err := state.Save(path, snap); err != nil {
		t.Fatalf("seeding state: %v", err)
	}
}
