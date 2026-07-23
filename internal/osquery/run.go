// Package osquery detects hardening drift: themis fixes that a prior
// `themis apply` run confirmed satisfied but that no longer hold. It
// re-verifies each one independently via osquery's built-in system
// tables rather than re-running the same Fix.Check logic that already
// passed once, so a config drifting back out from under themis (or a
// service stopping) is caught by a different code path than the one that
// applied it.
package osquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Elysium-Labs-EU/themis/internal/binpath"
)

// Row is one result row from an osqueryi query, keyed by column name —
// osquery's --json output always encodes values as strings regardless of
// the column's declared type.
type Row map[string]string

// ErrNotInstalled wraps the error Query returns when the osqueryi binary
// isn't present in a trusted dir. Source.Run treats this specific error
// as "drift detection isn't configured on this host" rather than a
// genuine failure.
var ErrNotInstalled = errors.New("osqueryi not installed")

// Query runs sql against osqueryi in --json mode and returns its result
// rows. sql is always one of the fixed queries in checks.go, never user
// input.
func Query(ctx context.Context, sql string) ([]Row, error) {
	bin, err := binpath.Resolve("osqueryi")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotInstalled, err)
	}

	cmd := exec.CommandContext(ctx, bin, "--json", sql) //nolint:gosec // bin resolved from binpath's trusted-dir allowlist; sql is a fixed query defined in checks.go, not user input
	cmd.Env = binpath.Environ(os.Environ())
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running osqueryi: %w", err)
	}

	var rows []Row
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		return nil, fmt.Errorf("parsing osqueryi output: %w", err)
	}
	return rows, nil
}
