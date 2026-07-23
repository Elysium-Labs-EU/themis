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

// withTrustFlags temporarily sets applyYes/applyTrust for a test, restoring
// their zero values on cleanup so later tests aren't affected.
func withTrustFlags(t *testing.T, yes bool, trust string) {
	t.Helper()
	origYes, origTrust := applyYes, applyTrust
	applyYes, applyTrust = yes, trust
	t.Cleanup(func() { applyYes, applyTrust = origYes, origTrust })
}

// withRegistry temporarily swaps fix.Registry so apply's mutating Apply()
// calls never touch the real host, restoring the original on cleanup.
func withRegistry(t *testing.T, reg map[string]fix.Fix) {
	t.Helper()
	orig := fix.Registry
	fix.Registry = reg
	t.Cleanup(func() { fix.Registry = orig })
}

func unsatisfiedFix(testID, description string, apply func() ([]byte, error)) fix.Fix {
	return fix.Fix{
		TestID:      testID,
		Description: description,
		Check:       func() (bool, error) { return false, nil },
		Apply:       apply,
		Revert:      func([]byte) error { return nil },
	}
}

// TestApplySavesStateAfterEveryFix is the regression test for issue #19:
// state.Save must be called after each successful Apply, not only once at
// the end of the loop, so a kill -9/SIGINT between two fixes can't lose
// rollback state for fixes that already succeeded.
func TestApplySavesStateAfterEveryFix(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	var sawEntriesBeforeB []string
	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "first fix", func() ([]byte, error) {
			return []byte("a"), nil
		}),
		"B-FIX": unsatisfiedFix("B-FIX", "second fix", func() ([]byte, error) {
			// resolveFixes/runApply process TestIDs in sorted order, so
			// A-FIX has already run by the time B-FIX's Apply executes.
			// If state was only saved at the end of the loop, this Load
			// would fail (no file yet) or return zero entries. Load
			// succeeding with A-FIX already present proves the save
			// happened incrementally, mid-loop.
			snap, err := state.Load(statePath)
			if err != nil {
				return nil, err
			}
			for _, e := range snap.Entries {
				sawEntriesBeforeB = append(sawEntriesBeforeB, e.TestID)
			}
			return []byte("b"), nil
		}),
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}

	if len(sawEntriesBeforeB) != 1 || sawEntriesBeforeB[0] != "A-FIX" {
		t.Fatalf("state visible to B-FIX's Apply = %v, want [A-FIX] (incremental save didn't happen before B-FIX ran)", sawEntriesBeforeB)
	}

	final, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("Load final state: %v", err)
	}
	if len(final.Entries) != 2 || final.Entries[0].TestID != "A-FIX" || final.Entries[1].TestID != "B-FIX" {
		t.Fatalf("final state entries = %+v, want [A-FIX B-FIX]", final.Entries)
	}
}

// TestApplyPreservesStateForFixesAppliedBeforeAFailure simulates the
// kill -9 scenario indirectly: a fix fails partway through the run, and
// the state for everything that succeeded before it must already be on
// disk (not just referenced in the error message).
func TestApplyPreservesStateForFixesAppliedBeforeAFailure(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "first fix", func() ([]byte, error) {
			return []byte("a"), nil
		}),
		"B-FIX": unsatisfiedFix("B-FIX", "second fix", func() ([]byte, error) {
			return nil, errors.New("boom")
		}),
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	err := runApply(cmd, statePath)
	if err == nil {
		t.Fatal("expected runApply to return an error when a fix fails")
	}

	snap, loadErr := state.Load(statePath)
	if loadErr != nil {
		t.Fatalf("Load after partial failure: %v (state for A-FIX should already be on disk)", loadErr)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("state after partial failure = %+v, want just A-FIX", snap.Entries)
	}
}

// TestApplyRecordsPartialRevertDataOnError is the regression test for
// issue #10: a Fix.Apply() that writes its target file and then fails at a
// later step (e.g. a service reload) may return non-nil revertData
// alongside the error. That revert data must still be recorded in
// state.json — otherwise the write is real but themis has no way to know
// about it, and rollback can never undo it.
func TestApplyRecordsPartialRevertDataOnError(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "first fix", func() ([]byte, error) {
			return []byte("a"), nil
		}),
		"B-FIX": unsatisfiedFix("B-FIX", "second fix, fails after a partial write", func() ([]byte, error) {
			return []byte("partial-b"), errors.New("post-write reload failed")
		}),
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	err := runApply(cmd, statePath)
	if err == nil {
		t.Fatal("expected runApply to return an error when a fix fails")
	}

	snap, loadErr := state.Load(statePath)
	if loadErr != nil {
		t.Fatalf("Load after partial failure: %v (partial revert data for B-FIX should already be on disk)", loadErr)
	}
	if len(snap.Entries) != 2 {
		t.Fatalf("state after partial failure = %+v, want 2 entries (A-FIX, B-FIX with partial revert data)", snap.Entries)
	}
	if snap.Entries[0].TestID != "A-FIX" {
		t.Fatalf("snap.Entries[0].TestID = %q, want A-FIX", snap.Entries[0].TestID)
	}
	bEntry := snap.Entries[1]
	if bEntry.TestID != "B-FIX" {
		t.Fatalf("snap.Entries[1].TestID = %q, want B-FIX", bEntry.TestID)
	}
	if string(bEntry.RevertData) != "partial-b" {
		t.Fatalf("B-FIX RevertData = %q, want %q", bEntry.RevertData, "partial-b")
	}
}

// TestApplyDoesNotRecordEntryWhenApplyFailsCleanly ensures the pre-existing
// behavior is unchanged for a Fix.Apply() that fails without returning any
// revert data: nothing new happened, so nothing new should be recorded.
func TestApplyDoesNotRecordEntryWhenApplyFailsCleanly(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": unsatisfiedFix("A-FIX", "fails with no revert data", func() ([]byte, error) {
			return nil, errors.New("boom, nothing written")
		}),
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	err := runApply(cmd, statePath)
	if err == nil {
		t.Fatal("expected runApply to return an error when a fix fails")
	}

	if _, loadErr := state.Load(statePath); loadErr == nil {
		t.Fatal("expected no state file to be written when Apply failed with no revert data")
	}
}

func TestApplyNothingToApplySkipsSave(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	withRegistry(t, map[string]fix.Fix{
		"A-FIX": {
			TestID:      "A-FIX",
			Description: "already satisfied",
			Check:       func() (bool, error) { return true, nil },
			Apply:       func() ([]byte, error) { t.Fatal("Apply should not run for a satisfied fix"); return nil, nil },
			Revert:      func([]byte) error { return nil },
		},
	})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	defer cmd.SetOut(nil)

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if _, err := state.Load(statePath); err == nil {
		t.Fatal("expected no state file to be written when nothing was applied")
	}
}

// trustAwareFix builds a fix whose Apply reports whatever CIDR SetTrust was
// last called with, so tests can assert on the resolved trust decision
// without touching a real jail.local.
func trustAwareFix(gotCIDR *string) fix.Fix {
	called := false
	return fix.Fix{
		TestID:      "T-FIX",
		Description: "trust-aware test fix",
		Check:       func() (bool, error) { return false, nil },
		SetTrust:    func(cidr string) { *gotCIDR = cidr; called = true },
		Apply: func() ([]byte, error) {
			if !called {
				return nil, errors.New("Apply ran without SetTrust having been called first")
			}
			return []byte("ok"), nil
		},
		Revert: func([]byte) error { return nil },
	}
}

// TestApplyTrustFlagSkipsPromptNonInteractive is the regression test for
// issue #20: --trust must resolve the exemption without ever reading from
// stdin, so unattended/cron applies of a trust-affecting fix don't hang.
func TestApplyTrustFlagSkipsPromptNonInteractive(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	withTrustFlags(t, false, "203.0.113.5/32")

	var gotCIDR string
	withRegistry(t, map[string]fix.Fix{"T-FIX": trustAwareFix(&gotCIDR)})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	cmd.SetIn(errReader{}) // reading from stdin here would be a bug — fail loudly if it happens
	defer func() { cmd.SetOut(nil); cmd.SetIn(nil) }()

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if gotCIDR != "203.0.113.5/32" {
		t.Fatalf("SetTrust received %q, want 203.0.113.5/32", gotCIDR)
	}
}

// errReader always fails on Read, so a test can assert a code path never
// touches stdin.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("unexpected stdin read") }

// TestApplyYesFlagAppliesWithNoExemption is the regression test for issue
// #20's --yes flag: unattended runs must proceed without an exemption
// rather than blocking on a prompt.
func TestApplyYesFlagAppliesWithNoExemption(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	withTrustFlags(t, true, "")

	var gotCIDR string
	withRegistry(t, map[string]fix.Fix{"T-FIX": trustAwareFix(&gotCIDR)})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	cmd.SetIn(errReader{})
	defer func() { cmd.SetOut(nil); cmd.SetIn(nil) }()

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if gotCIDR != "" {
		t.Fatalf("SetTrust received %q, want \"\" (no exemption)", gotCIDR)
	}
}

// TestApplyInteractivePromptCurrentConnection covers answering "c" at the
// interactive prompt to exempt the detected SSH_CONNECTION client IP.
func TestApplyInteractivePromptCurrentConnection(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	withTrustFlags(t, false, "")
	t.Setenv("SSH_CONNECTION", "203.0.113.5 51234 198.51.100.9 22")

	var gotCIDR string
	withRegistry(t, map[string]fix.Fix{"T-FIX": trustAwareFix(&gotCIDR)})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	cmd.SetIn(strings.NewReader("c\n"))
	defer func() { cmd.SetOut(nil); cmd.SetIn(nil) }()

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if gotCIDR != "203.0.113.5/32" {
		t.Fatalf("SetTrust received %q, want 203.0.113.5/32", gotCIDR)
	}
	if !strings.Contains(buf.String(), "T-FIX") {
		t.Errorf("expected the trust prompt to mention T-FIX, got %q", buf.String())
	}
}

// TestApplyInteractivePromptCustomCIDR covers typing a raw CIDR at the
// prompt instead of using "c" or "n".
func TestApplyInteractivePromptCustomCIDR(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	withTrustFlags(t, false, "")

	var gotCIDR string
	withRegistry(t, map[string]fix.Fix{"T-FIX": trustAwareFix(&gotCIDR)})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	cmd.SetIn(strings.NewReader("198.51.100.0/24\n"))
	defer func() { cmd.SetOut(nil); cmd.SetIn(nil) }()

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if gotCIDR != "198.51.100.0/24" {
		t.Fatalf("SetTrust received %q, want 198.51.100.0/24", gotCIDR)
	}
}

// TestApplyInteractivePromptBlankSkipsExemption covers the safe default:
// a bare Enter applies with no exemption.
func TestApplyInteractivePromptBlankSkipsExemption(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	withTrustFlags(t, false, "")

	var gotCIDR string
	withRegistry(t, map[string]fix.Fix{"T-FIX": trustAwareFix(&gotCIDR)})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	cmd.SetIn(strings.NewReader("\n"))
	defer func() { cmd.SetOut(nil); cmd.SetIn(nil) }()

	if err := runApply(cmd, statePath); err != nil {
		t.Fatalf("runApply: %v", err)
	}
	if gotCIDR != "" {
		t.Fatalf("SetTrust received %q, want \"\" (no exemption)", gotCIDR)
	}
}

func TestResolveTrustAnswer(t *testing.T) {
	cases := []struct {
		name        string
		answer      string
		currentCIDR string
		want        string
		wantErr     bool
	}{
		{"blank means no exemption", "\n", "203.0.113.5/32", "", false},
		{"n means no exemption", "n\n", "203.0.113.5/32", "", false},
		{"c uses current connection", "c\n", "203.0.113.5/32", "203.0.113.5/32", false},
		{"c with no current connection errors", "c\n", "", "", true},
		{"raw CIDR passes through", "198.51.100.0/24\n", "", "198.51.100.0/24", false},
		{"bare IPv4 widens to /32", "198.51.100.7\n", "", "198.51.100.7/32", false},
		{"garbage errors", "not-an-ip\n", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTrustAnswer(tc.answer, tc.currentCIDR)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resolveTrustAnswer(%q, %q) error = %v, wantErr %v", tc.answer, tc.currentCIDR, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("resolveTrustAnswer(%q, %q) = %q, want %q", tc.answer, tc.currentCIDR, got, tc.want)
			}
		})
	}
}

func TestNormalizeCIDR(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"already a CIDR", "198.51.100.0/24", "198.51.100.0/24", false},
		{"bare IPv4", "198.51.100.7", "198.51.100.7/32", false},
		{"bare IPv6", "2001:db8::5", "2001:db8::5/128", false},
		{"invalid", "not-an-ip", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeCIDR(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("normalizeCIDR(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("normalizeCIDR(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestResolveTrustedCIDRRejectsInvalidTrustFlag ensures a malformed --trust
// value fails fast with a clear error instead of silently exempting nothing.
func TestResolveTrustedCIDRRejectsInvalidTrustFlag(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	withTrustFlags(t, false, "not-a-cidr")

	var gotCIDR string
	withRegistry(t, map[string]fix.Fix{"T-FIX": trustAwareFix(&gotCIDR)})

	buf := &bytes.Buffer{}
	cmd := applyCmd
	cmd.SetOut(buf)
	cmd.SetIn(errReader{})
	defer func() { cmd.SetOut(nil); cmd.SetIn(nil) }()

	if err := runApply(cmd, statePath); err == nil {
		t.Fatal("expected runApply to fail with an invalid --trust value")
	}
}
