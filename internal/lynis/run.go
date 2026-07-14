package lynis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"codeberg.org/Elysium_Labs/themis/internal/ui"
)

// ReportPath is the default location Lynis writes its machine-readable
// report to.
const ReportPath = "/var/log/lynis-report.dat"

// sbinFallbacks are common install locations for lynis that a non-root
// user's $PATH often excludes (e.g. Debian puts it in /usr/sbin, which
// only root's PATH includes by default) — sudo works, a bare `themis
// check` doesn't, and the resulting "not installed" error is wrong: the
// binary is there, just not found. Check these before giving up.
var sbinFallbacks = []string{"/usr/sbin/lynis", "/sbin/lynis"}

// lynisPath resolves the lynis binary: $PATH first, falling back to
// common sbin locations non-root PATHs often omit.
func lynisPath() (string, error) {
	return lynisPathWith(exec.LookPath, fileExists, sbinFallbacks)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// lynisPathWith does the actual resolution, with lookPath/exists/fallbacks
// parameterized so the fallback behavior is testable without needing a
// real /usr/sbin/lynis on the test machine.
func lynisPathWith(lookPath func(string) (string, error), exists func(string) bool, fallbacks []string) (string, error) {
	if p, err := lookPath("lynis"); err == nil {
		return p, nil
	}
	for _, p := range fallbacks {
		if exists(p) {
			return p, nil
		}
	}
	return "", exec.ErrNotFound
}

// Audit runs `lynis audit system` and returns the parsed findings from
// the report it writes to ReportPath.
func Audit(ctx context.Context) ([]Finding, error) {
	lynisBin, err := lynisPath()
	if err != nil {
		return nil, &ui.UserError{
			Err:  errors.New("lynis not found"),
			Hint: "apt install lynis (already installed but not on PATH? try: sudo themis check)",
		}
	}

	cmd := exec.CommandContext(ctx, lynisBin, "audit", "system", "--quiet") //nolint:gosec // lynisBin resolved above from PATH or a fixed allowlist, not user input
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		// Lynis exits non-zero when it has warnings/suggestions; only
		// treat a missing report as a hard failure here.
		if !errors.As(runErr, &exitErr) {
			return nil, fmt.Errorf("running lynis audit: %w", runErr)
		}
	}

	f, err := os.Open(ReportPath)
	if err != nil {
		return nil, fmt.Errorf("opening lynis report %s: %w", ReportPath, err)
	}
	defer func() { _ = f.Close() }()

	findings, err := ParseReport(f)
	if err != nil {
		return nil, fmt.Errorf("parsing lynis report %s: %w", ReportPath, err)
	}
	return findings, nil
}
