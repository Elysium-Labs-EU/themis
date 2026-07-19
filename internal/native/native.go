// Package native implements themis-native audit checks: things Lynis
// handles poorly or doesn't check at all, with no external tool
// dependency. It plugs into audit.Run as one more audit.Source alongside
// Lynis.
package native

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
	"codeberg.org/Elysium_Labs/themis/internal/binpath"
)

// Source runs themis's own native checks as a pluggable audit.Source.
type Source struct{}

// NewSource returns a themis-native audit.Source.
func NewSource() Source { return Source{} }

// Name identifies this source as "themis".
func (Source) Name() string { return "themis" }

// Run executes every native check and returns their findings.
func (Source) Run(ctx context.Context) ([]audit.Finding, error) {
	var findings []audit.Finding

	f2b, err := fail2banFinding(ctx)
	if err != nil {
		return nil, err
	}
	if f2b != nil {
		findings = append(findings, *f2b)
	}

	uu, err := unattendedUpgradesFinding(ctx)
	if err != nil {
		return nil, err
	}
	if uu != nil {
		findings = append(findings, *uu)
	}

	return findings, nil
}

// runCmd runs name with args, returning combined output wrapped into the
// error on failure so callers get actionable context. Takes ctx (unlike
// internal/fix's runCmd) so a themis check honors audit.Run's
// cancellation instead of running unbounded. name is resolved against
// binpath's trusted dirs rather than $PATH — themis check can run as
// root, and a bare name search would let anything planted earlier in an
// inherited $PATH execute in its place. The child's own $PATH is pinned
// the same way, in case it shells out further internally.
func runCmd(ctx context.Context, name string, args ...string) error {
	bin, err := binpath.Resolve(name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin resolved from a fixed trusted-dir allowlist, not $PATH or user input
	cmd.Env = binpath.Environ(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// packageInstalled reports whether name is installed via dpkg. Not
// reused from internal/fix, whose equivalent doesn't take a context —
// this one honors audit.Run's cancellation via runCmd above.
func packageInstalled(ctx context.Context, name string) bool {
	return runCmd(ctx, "dpkg", "-s", name) == nil
}
