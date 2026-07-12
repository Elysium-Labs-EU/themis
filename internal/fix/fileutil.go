package fix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readFileOrEmpty reads path, reporting whether it existed. A missing
// file is not an error — callers treat "didn't exist" as meaningful
// revert state (Revert should remove the file, not restore empty
// content).
func readFileOrEmpty(path string) (content []byte, existed bool, err error) {
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
// error on failure so callers get actionable context.
func runCmd(name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // name/args are fixed literals at each call site, not user input
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runCmdOutput is like runCmd but returns stdout+stderr on success too.
func runCmdOutput(name string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // name/args are fixed literals at each call site, not user input
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("running %s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

func packageInstalled(name string) bool {
	return runCmd("dpkg", "-s", name) == nil
}
