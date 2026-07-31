package osquery

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// TestQueryWrapsErrNotInstalledWhenOsqueryiMissing covers Query's
// binpath.Resolve failure path: Source.Run treats ErrNotInstalled as
// "drift detection isn't configured" rather than a genuine failure, so
// the wrap must be detectable via errors.Is on whatever Query returns.
func TestQueryWrapsErrNotInstalledWhenOsqueryiMissing(t *testing.T) {
	if _, err := exec.LookPath("osqueryi"); err == nil {
		t.Skip("osqueryi is installed on this host; skipping the missing-binary path")
	}

	rows, err := Query(context.Background(), "select 1")
	if rows != nil {
		t.Errorf("rows = %v, want nil on error", rows)
	}
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Query error = %v, want it to wrap ErrNotInstalled", err)
	}
}
