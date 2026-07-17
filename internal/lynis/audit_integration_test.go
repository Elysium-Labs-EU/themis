//go:build integration

package lynis

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// Integration tests here drive the REAL lynis binary end to end. They
// require:
//   - Linux
//   - root (lynis audit system needs it, and ReportPath is root-owned)
//   - lynis installed on PATH or in the /usr/sbin//sbin fallbacks the code
//     already resolves
//
// Run via: make test-integration
//   or on OrbStack: orb run -m debian -u root bash -lc \
//     "export PATH=/usr/local/go/bin:\$PATH; cd <repo> && go test ./internal/lynis/... -tags integration -v"
//
// Anything missing (non-Linux, non-root, no lynis) t.Skip's rather than
// failing, so the same suite is safe to run on a laptop or a CI runner
// without lynis.

func requireLynis(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux")
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	// lynisPath resolves $PATH first, then the /usr/sbin, /sbin fallbacks the
	// production code handles. Skip (not fail) when the binary is genuinely
	// absent — the CI runner may not have it.
	if _, err := lynisPath(); err != nil {
		t.Skip("lynis not installed (not on PATH or /usr/sbin//sbin)")
	}
}

// TestAuditIntegration runs a real `lynis audit system` and asserts the
// report it writes is parsed into findings. Uses the Quick profile to keep
// the scan light on a shared/CI host. A non-zero lynis exit (which lynis
// returns whenever it has suggestions/warnings — i.e. almost always) must
// NOT surface as an error: Audit tolerates it and still parses the report.
func TestAuditIntegration(t *testing.T) {
	requireLynis(t)

	// A file mtime strictly after this instant proves the run produced a
	// fresh report rather than us reading a stale one from a prior audit.
	// Back off a second to stay clear of filesystem mtime granularity.
	before := time.Now().Add(-time.Second)

	findings, err := Audit(t.Context(), Options{Quick: true})
	if err != nil {
		// If Audit returned an error it either couldn't run lynis or the
		// report was missing/unparseable — a non-zero lynis exit alone must
		// not land here.
		t.Fatalf("Audit returned an error (a non-zero lynis exit should be tolerated): %v", err)
	}

	// A full/quick audit always yields at least one suggestion on a stock
	// host; zero findings means the report didn't parse.
	if len(findings) < 1 {
		t.Fatalf("expected >=1 parsed finding, got %d", len(findings))
	}

	// At least one finding should carry a real Lynis test ID.
	hasTestID := false
	for _, f := range findings {
		if f.TestID != "" {
			hasTestID = true
			break
		}
	}
	if !hasTestID {
		t.Error("no finding carried a non-empty TestID; report likely mis-parsed")
	}

	// The report must exist and be freshly written by this run.
	info, err := os.Stat(ReportPath)
	if err != nil {
		t.Fatalf("stat %s after audit: %v", ReportPath, err)
	}
	if !info.ModTime().After(before) {
		t.Errorf("report %s mtime %v is not newer than %v — audit did not write a fresh report", ReportPath, info.ModTime(), before)
	}
}
