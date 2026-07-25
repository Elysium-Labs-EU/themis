//go:build integration

package openscap

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/binpath"
)

// sscContentGlob matches SCAP Security Guide datastreams as installed by
// Debian's scap-security-guide(-debian) package, whatever the Debian
// release (ssg-debian11-ds.xml, ssg-debian12-ds.xml, ...).
const ssgContentGlob = "/usr/share/xml/scap/ssg/content/ssg-debian*-ds.xml"

// Integration tests here drive the REAL oscap binary end to end. They
// require:
//   - Linux
//   - root (most CIS/DISA rules need it to read protected config)
//   - oscap installed in one of binpath's trusted dirs
//   - scap-security-guide(-debian) content installed
//
// Run via: make test-integration
//
// Anything missing (non-Linux, non-root, no oscap, no SSG content) t.Skip's
// rather than failing, so the same suite is safe to run on a laptop or a
// CI runner without OpenSCAP.
func requireOscap(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux")
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	if _, err := binpath.Resolve("oscap"); err != nil {
		t.Skip("oscap not installed (not in a trusted dir: /usr/sbin, /sbin, ...)")
	}
	matches, err := filepath.Glob(ssgContentGlob)
	if err != nil || len(matches) == 0 {
		t.Skip("no scap-security-guide content installed (apt install scap-security-guide-debian)")
	}
	return matches[0]
}

// TestAuditIntegration runs a real `oscap xccdf eval` against installed
// SSG content and asserts its stdout is parsed into findings. A non-zero
// oscap exit (which oscap returns whenever any rule fails — i.e. almost
// always on a stock host) must NOT surface as an error: Audit tolerates
// it and still parses the output.
func TestAuditIntegration(t *testing.T) {
	content := requireOscap(t)

	findings, err := Audit(t.Context(), Options{ContentPath: content})
	if err != nil {
		t.Fatalf("Audit returned an error (a non-zero oscap exit should be tolerated): %v", err)
	}

	// SSG content always yields at least one failed or unchecked rule on a
	// stock host; zero findings means the output didn't parse.
	if len(findings) < 1 {
		t.Fatalf("expected >=1 parsed finding, got %d", len(findings))
	}

	hasTestID := false
	for _, f := range findings {
		if f.TestID != "" {
			hasTestID = true
			break
		}
	}
	if !hasTestID {
		t.Error("no finding carried a non-empty TestID; output likely mis-parsed")
	}
}
