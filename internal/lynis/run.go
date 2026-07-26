package lynis

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"syscall"

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
	// SkipIfUnchanged skips re-running lynis and reuses the last report's
	// findings when none of the config files or the package list lynis
	// cares about have changed since the last full scan. Default (false)
	// always runs a full scan — this is an opt-in for resource-
	// constrained or stateful hosts that don't want to pay for a lynis
	// run when nothing changed.
	SkipIfUnchanged bool
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
		return nil, lynisNotFoundError(binpath.TrustedDirs)
	}

	if findings, ok := trySkip(opts, fingerprintPaths, dpkgStatusPath, FingerprintPath, ReportPath); ok {
		return findings, nil
	}

	if runErr := runLynisAudit(ctx, lynisBin, opts); runErr != nil {
		return nil, runErr
	}

	persistFingerprint(opts, fingerprintPaths, dpkgStatusPath, FingerprintPath)

	return readReport(ReportPath)
}

// lynisNotFoundError builds the error returned when lynis can't be resolved
// from trustedDirs. The hint names those dirs directly rather than
// suggesting a specific package manager (e.g. apt), since themis runs on
// Debian, Fedora, Alpine, and other distros whose install locations and
// package managers all differ. Pure — no I/O.
func lynisNotFoundError(trustedDirs []string) *ui.UserError {
	return &ui.UserError{
		Err:  errors.New("lynis not found"),
		Hint: "install lynis with your distro's package manager and ensure it's on one of: " + strings.Join(trustedDirs, ", "),
	}
}

// trySkip reports whether a lynis re-scan can be skipped per
// opts.SkipIfUnchanged, returning the last report's findings when it can.
// ok is false whenever a full run is still needed: the option is off, the
// fingerprint doesn't match (or errored reading it — a fingerprinting
// problem isn't fatal, it just means we can't prove nothing changed), or
// the cached report itself can no longer be read.
func trySkip(opts Options, configPaths []string, pkgListPath, fingerprintCachePath, reportPath string) ([]Finding, bool) {
	if !opts.SkipIfUnchanged {
		return nil, false
	}
	skip, err := shouldSkip(configPaths, pkgListPath, fingerprintCachePath, reportPath, scanProfile(opts.Quick))
	if err != nil || !skip {
		return nil, false
	}
	findings, err := readReport(reportPath)
	if err != nil {
		return nil, false
	}
	return findings, true
}

// persistFingerprint saves the post-scan fingerprint for a later
// SkipIfUnchanged run to compare against, per opts.SkipIfUnchanged. Best-
// effort: an error here isn't fatal to the scan that just ran — it just
// means the next run won't skip and will pay for another full scan
// instead.
func persistFingerprint(opts Options, configPaths []string, pkgListPath, fingerprintCachePath string) {
	if !opts.SkipIfUnchanged {
		return
	}
	if fp, err := readFingerprint(configPaths, pkgListPath, scanProfile(opts.Quick)); err == nil {
		_ = saveFingerprint(fingerprintCachePath, fp)
	}
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

// readReport opens the lynis report at path and returns its parsed
// findings. Like state.json (internal/state/state.go), the report sits
// between when lynis (running as root) writes it and when themis reads
// it back, so it's treated as hostile: another user with write access to
// its directory could otherwise plant a symlink or swap in a report of
// their own choosing ahead of this root-privileged read, whose contents
// flow straight into fix/verdict classification. O_NOFOLLOW rejects a
// symlink at path outright, and the ownership/mode check runs against
// the opened fd so there's no window between check and read.
func readReport(path string) ([]Finding, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0) //nolint:gosec // path is a fixed report-file constant; O_NOFOLLOW rejects symlinks
	if err != nil {
		return nil, fmt.Errorf("opening lynis report %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("opening lynis report %s: %w", path, err)
	}
	if verifyErr := verifyReportOwnerAndMode(info, os.Geteuid()); verifyErr != nil {
		return nil, fmt.Errorf("lynis report %s failed integrity check: %w", path, verifyErr)
	}

	findings, err := ParseReport(f)
	if err != nil {
		return nil, fmt.Errorf("parsing lynis report %s: %w", path, err)
	}
	return findings, nil
}

// verifyReportOwnerAndMode rejects a report file that isn't owned by
// wantUID or that grants group/other write access — either means someone
// besides the user running themis could have written or swapped it.
// World-read is left alone (lynis's own default mode for a log file);
// only write access crosses the trust boundary. Pure — info and wantUID
// are already-resolved inputs, no I/O here.
func verifyReportOwnerAndMode(info fs.FileInfo, wantUID int) error {
	if !info.Mode().IsRegular() {
		return errors.New("not a regular file")
	}
	if perm := info.Mode().Perm(); perm&0o022 != 0 {
		return fmt.Errorf("mode %04o is writable by group/other", perm)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot determine file owner")
	}
	if int(stat.Uid) != wantUID {
		return fmt.Errorf("owned by uid %d, want %d", stat.Uid, wantUID)
	}
	return nil
}
