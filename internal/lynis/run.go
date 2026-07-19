package lynis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Elysium-Labs-EU/themis/internal/binpath"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// ReportPath is the default location Lynis writes its machine-readable
// report to.
const ReportPath = "/var/log/lynis-report.dat"

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

// priorityWrap prefixes bin/args with ionice and/or nice, when present in
// a trusted dir, so a full audit doesn't starve other work on resource-
// constrained or stateful hosts. It doesn't reduce total CPU time, only
// priority. Falls back to running bin directly if neither tool is found
// (e.g. ionice doesn't exist on macOS). resolve is parameterized (rather
// than calling binpath.Resolve directly) so tests can drive it without
// touching the filesystem; production wires binpath.Resolve, never
// exec.LookPath — themis runs as root, and a $PATH search for "nice"
// could be shadowed by something planted earlier in an inherited PATH.
// Pure given resolve — no I/O itself.
func priorityWrap(resolve func(string) (string, error), bin string, args []string) (string, []string) {
	cmdArgs := append([]string{bin}, args...)
	if p, err := resolve("nice"); err == nil {
		cmdArgs = append([]string{p, "-n", "19"}, cmdArgs...)
	}
	if p, err := resolve("ionice"); err == nil {
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

	lynisBin, err := binpath.Resolve("lynis")
	if err != nil {
		return nil, &ui.UserError{
			Err:  errors.New("lynis not found"),
			Hint: "apt install lynis (already installed but not in a trusted dir? check /usr/sbin, /sbin)",
		}
	}

	if runErr := runLynisAudit(ctx, lynisBin, opts); runErr != nil {
		return nil, runErr
	}

	return readReport(ReportPath)
}

// runLynisAudit runs `lynis audit system`, tolerating the non-zero exit
// lynis returns when it merely has warnings/suggestions — only a genuine
// failure to run (e.g. the binary vanished) is treated as an error.
// Lynis itself shells out to dpkg, sysctl, and more as part of its audit,
// so cmd.Env pins its $PATH to binpath's trusted dirs too — otherwise
// those grandchild execs would still resolve through the inherited
// (and, since themis runs as root, potentially attacker-influenced) $PATH,
// even though the lynis binary itself was resolved safely above.
func runLynisAudit(ctx context.Context, lynisBin string, opts Options) error {
	runBin, runArgs := priorityWrap(binpath.Resolve, lynisBin, lynisArgs(opts))
	cmd := exec.CommandContext(ctx, runBin, runArgs...) //nolint:gosec // runBin resolved above from a fixed trusted-dir allowlist, not $PATH or user input
	cmd.Env = binpath.Environ(os.Environ())
	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return fmt.Errorf("running lynis audit: %w", runErr)
	}
	return nil
}

// readReport opens the lynis report at path and returns its parsed findings.
func readReport(path string) ([]Finding, error) {
	f, err := os.Open(path) //nolint:gosec // path is a fixed report-file constant at the call site
	if err != nil {
		return nil, fmt.Errorf("opening lynis report %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	findings, err := ParseReport(f)
	if err != nil {
		return nil, fmt.Errorf("parsing lynis report %s: %w", path, err)
	}
	return findings, nil
}
