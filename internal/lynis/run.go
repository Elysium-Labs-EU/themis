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

// sbinFallbacks are common install locations for lynis that root's $PATH
// can still exclude on some distros (e.g. Debian puts it in /usr/sbin) —
// the resulting "not installed" error would be wrong: the binary is
// there, just not found. Check these before giving up.
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

// Options configures how Audit runs lynis.
type Options struct {
	// Quick runs lynis's own --quick profile, which skips some tests in
	// exchange for a faster, lighter scan. Default (false) is a full
	// audit.
	Quick bool
}

// lynisArgs builds the `lynis audit system` argument list for the given
// options. Pure — no I/O.
func lynisArgs(opts Options) []string {
	args := []string{"audit", "system", "--quiet"}
	if opts.Quick {
		args = append(args, "--quick")
	}
	return args
}

// priorityWrap prefixes bin/args with ionice and/or nice, when present on
// PATH, so a full audit doesn't starve other work on resource-constrained
// or stateful hosts. It doesn't reduce total CPU time, only priority.
// Falls back to running bin directly if neither tool is found (e.g.
// ionice doesn't exist on macOS). Pure given lookPath — no I/O itself.
func priorityWrap(lookPath func(string) (string, error), bin string, args []string) (string, []string) {
	cmdArgs := append([]string{bin}, args...)
	if p, err := lookPath("nice"); err == nil {
		cmdArgs = append([]string{p, "-n", "19"}, cmdArgs...)
	}
	if p, err := lookPath("ionice"); err == nil {
		cmdArgs = append([]string{p, "-c3"}, cmdArgs...)
	}
	return cmdArgs[0], cmdArgs[1:]
}

// Audit runs `lynis audit system` and returns the parsed findings from
// the report it writes to ReportPath.
func Audit(ctx context.Context, opts Options) ([]Finding, error) {
	// lynis audit system needs root to run its full scan and to write
	// ReportPath (often owned root:root from a prior run either way).
	// Check euid before paying for the multi-minute scan, rather than
	// discovering the permission problem only once we try to open the
	// report afterwards.
	if os.Geteuid() != 0 {
		return nil, &ui.UserError{
			Err:  errors.New("themis check requires root to run and read the lynis audit"),
			Hint: "sudo themis check",
		}
	}

	lynisBin, err := lynisPath()
	if err != nil {
		return nil, &ui.UserError{
			Err:  errors.New("lynis not found"),
			Hint: "apt install lynis (already installed but not on PATH? check /usr/sbin, /sbin)",
		}
	}

	runBin, runArgs := priorityWrap(exec.LookPath, lynisBin, lynisArgs(opts))
	cmd := exec.CommandContext(ctx, runBin, runArgs...) //nolint:gosec // runBin resolved above from PATH or a fixed allowlist, not user input
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
