package openscap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Elysium-Labs-EU/themis/internal/binpath"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

// Options configures how Audit runs oscap.
type Options struct {
	// ContentPath is the SCAP/XCCDF datastream to evaluate, e.g. the
	// SCAP Security Guide content installed by scap-security-guide(-debian)
	// at /usr/share/xml/scap/ssg/content/ssg-debian12-ds.xml. Required —
	// oscap has no useful default content to fall back to.
	ContentPath string
	// Profile is the XCCDF profile ID to evaluate, e.g.
	// "xccdf_org.ssgproject.content_profile_cis_level1_server". Empty
	// uses the datastream's own default profile.
	Profile string
}

// oscapArgs builds the `oscap xccdf eval` argument list for the given
// options. Pure — no I/O.
func oscapArgs(opts Options) []string {
	args := []string{"xccdf", "eval"}
	if opts.Profile != "" {
		args = append(args, "--profile", opts.Profile)
	}
	return append(args, opts.ContentPath)
}

// Audit runs `oscap xccdf eval` against opts.ContentPath and returns the
// findings parsed from its stdout.
func Audit(ctx context.Context, opts Options) ([]Finding, error) {
	if opts.ContentPath == "" {
		return nil, &ui.UserError{
			Err:  errors.New("openscap requires a SCAP content file"),
			Hint: "install scap-security-guide (apt install scap-security-guide-debian) and pass --scap-content /usr/share/xml/scap/ssg/content/ssg-debian12-ds.xml",
		}
	}
	// oscap needs root to read protected config (sshd_config, shadow,
	// ...) for most CIS/DISA rules to evaluate accurately, same as lynis.
	if os.Geteuid() != 0 {
		return nil, &ui.UserError{
			Err:  errors.New("themis check requires root to run the openscap audit"),
			Hint: "sudo themis check",
		}
	}

	oscapBin, err := binpath.Resolve("oscap")
	if err != nil {
		return nil, &ui.UserError{
			Err:  errors.New("oscap not found"),
			Hint: "apt install libopenscap8 (already installed but not in a trusted dir? check /usr/bin, /usr/local/bin)",
		}
	}

	out, runErr := runOscapEval(ctx, oscapBin, opts)
	if runErr != nil {
		return nil, runErr
	}

	findings, err := ParseOutput(strings.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("parsing oscap output: %w", err)
	}
	return findings, nil
}

// runOscapEval runs `oscap xccdf eval` and returns its stdout, tolerating
// the non-zero exit oscap returns whenever any rule fails (analogous to
// lynis's non-zero exit on suggestions/warnings) — only a genuine failure
// to run is treated as an error. Env is pinned to binpath's trusted dirs,
// same rationale as lynis: oscap is a root-run audit tool and shouldn't
// resolve any further grandchild command through an inherited (and,
// since themis runs as root, potentially attacker-influenced) $PATH.
func runOscapEval(ctx context.Context, oscapBin string, opts Options) (string, error) {
	cmd := exec.CommandContext(ctx, oscapBin, oscapArgs(opts)...) //nolint:gosec // oscapBin resolved above from a fixed trusted-dir allowlist, not $PATH or user input
	cmd.Env = binpath.Environ(os.Environ())
	out, runErr := cmd.Output()
	if runErr == nil {
		return string(out), nil
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return "", fmt.Errorf("running oscap xccdf eval: %w", runErr)
	}
	return string(out), nil
}
