// Package native implements themis-native audit checks: things Lynis
// handles poorly or doesn't check at all, with no external tool
// dependency. It plugs into audit.Run as one more audit.Source alongside
// Lynis.
package native

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
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
// cancellation instead of running unbounded.
func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // name/args are fixed literals at each call site, not user input
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
