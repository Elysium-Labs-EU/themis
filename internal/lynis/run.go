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

// Audit runs `lynis audit system` and returns the parsed findings from
// the report it writes to ReportPath.
func Audit(ctx context.Context) ([]Finding, error) {
	cmd := exec.CommandContext(ctx, "lynis", "audit", "system", "--quiet")
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		// Lynis exits non-zero when it has warnings/suggestions; only
		// treat a missing binary or report as a hard failure.
		if !errors.As(err, &exitErr) {
			if errors.Is(err, exec.ErrNotFound) {
				return nil, &ui.UserError{
					Err:  errors.New("lynis is not installed"),
					Hint: "apt install lynis",
				}
			}
			return nil, fmt.Errorf("running lynis audit: %w", err)
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
