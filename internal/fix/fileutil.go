package fix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Elysium-Labs-EU/themis/internal/binpath"
)

// cmdRunner runs a command, reporting failure as an error. Real fixes wire
// runCmd; tests inject a fake to drive Apply/Revert without touching the host.
type cmdRunner func(name string, args ...string) error

// outputRunner runs a command and returns its combined output. Real fixes
// wire runCmdOutput; tests inject a fake.
type outputRunner func(name string, args ...string) (string, error)

// pkgChecker reports whether a package is installed. Real fixes wire
// packageInstalled; tests inject a fake.
type pkgChecker func(name string) bool

// ReadFileOrEmpty reads path, reporting whether it existed. A missing
// file is not an error — callers treat "didn't exist" as meaningful
// revert state (Revert should remove the file, not restore empty
// content). Exported for internal/native, which reads the same config
// files for its findings.
func ReadFileOrEmpty(path string) (content []byte, existed bool, err error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is always one of our fixed config-file constants, not user input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading %s: %w", path, err)
	}
	return b, true, nil
}

func writeFile(path string, content []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, content, perm); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

// runCmd runs name with args, returning combined output wrapped into the
// error on failure so callers get actionable context. name is resolved
// against binpath's trusted dirs rather than $PATH, so a fixed literal
// like "systemctl" can't be shadowed by something planted earlier in an
// inherited (and, since themis runs as root, potentially attacker-
// influenced) $PATH. The child's own $PATH is pinned the same way, so a
// tool that shells out internally (e.g. apt-get invoking dpkg) can't be
// tricked into a planted binary either.
func runCmd(name string, args ...string) error {
	bin, err := binpath.Resolve(name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", name, err)
	}
	cmd := exec.CommandContext(context.Background(), bin, args...) //nolint:gosec // bin resolved from a fixed trusted-dir allowlist, not $PATH or user input
	cmd.Env = binpath.Environ(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runCmdOutput is like runCmd but returns stdout+stderr on success too.
func runCmdOutput(name string, args ...string) (string, error) {
	bin, err := binpath.Resolve(name)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", name, err)
	}
	cmd := exec.CommandContext(context.Background(), bin, args...) //nolint:gosec // bin resolved from a fixed trusted-dir allowlist, not $PATH or user input
	cmd.Env = binpath.Environ(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("running %s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

func packageInstalled(name string) bool {
	return runCmd("dpkg", "-s", name) == nil
}

// revertDrifted reports whether the file at path currently differs from
// applied — the content a Fix's Apply is expected to have written — and if
// so returns the standard "changed since apply" advisory for a Fix's
// RevertWarn. A missing file has nothing left to discard, so that case
// reports no drift; Revert already knows how to handle a missing file on
// its own.
func revertDrifted(path, applied string) (message string, detected bool, err error) {
	current, existed, err := ReadFileOrEmpty(path)
	if err != nil {
		return "", false, err
	}
	if !existed || string(current) == applied {
		return "", false, nil
	}
	return fmt.Sprintf(
		"%s has changed since apply — reverting now would discard those changes; review the file, then rerun rollback with --force to revert anyway",
		path,
	), true, nil
}
