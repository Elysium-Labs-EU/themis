package native

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDpkgStatusInstalled(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{"installed", "install ok installed", true},
		{"installed with trailing newline", "install ok installed\n", true},
		{"removed but not purged, conffiles remain", "deinstall ok config-files", false},
		{"half-installed", "install ok half-installed", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dpkgStatusInstalled(tc.status); got != tc.want {
				t.Errorf("dpkgStatusInstalled(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// stubResult is a canned (output, err) pair a stubRunner returns for one
// command name.
type stubResult struct {
	err error
	out string
}

// stubRunner returns a commandRunner keyed by command name so tests can
// drive fail2banFinding, unattendedUpgradesFinding, and run without a
// live host. A command name missing from results fails the test loudly
// rather than silently returning a zero value, so a test only wires the
// calls it actually expects.
func stubRunner(t *testing.T, results map[string]stubResult) commandRunner {
	t.Helper()
	return func(_ context.Context, name string, _ ...string) (string, error) {
		res, ok := results[name]
		if !ok {
			t.Fatalf("unexpected command: %s", name)
		}
		return res.out, res.err
	}
}

func TestRunCmdOutputRunsRealBinary(t *testing.T) {
	// "echo" is a standard coreutils binary present at /bin or /usr/bin
	// on any Linux or macOS dev/CI box — exercises the real
	// binpath.Resolve + exec.CommandContext path, not just a fake.
	out, err := runCmdOutput(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("runCmdOutput(echo): %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("output = %q, want %q", out, "hello")
	}
}

func TestRunCmdOutputWrapsFailure(t *testing.T) {
	_, err := runCmdOutput(context.Background(), "false")
	if err == nil {
		t.Fatal("expected an error from a command that exits non-zero")
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("error = %q, want it to mention the command name", err.Error())
	}
}

func TestRunCmdOutputUnresolvedBinary(t *testing.T) {
	_, err := runCmdOutput(context.Background(), "definitely-not-a-real-binary-xyz123")
	if err == nil {
		t.Fatal("expected an error for a binary not in any trusted dir")
	}
	if !strings.Contains(err.Error(), "resolving") {
		t.Errorf("error = %q, want it to mention resolving", err.Error())
	}
}

func TestRunCmdSuccess(t *testing.T) {
	if err := runCmd(context.Background(), runCmdOutput, "true"); err != nil {
		t.Errorf("runCmd(true) = %v, want nil", err)
	}
}

func TestRunCmdFailurePropagatesRunnerError(t *testing.T) {
	boom := errors.New("boom")
	runner := stubRunner(t, map[string]stubResult{"whatever": {err: boom}})

	if err := runCmd(context.Background(), runner, "whatever"); !errors.Is(err, boom) {
		t.Errorf("runCmd = %v, want %v", err, boom)
	}
}

func TestPackageInstalled(t *testing.T) {
	cases := []struct {
		result stubResult
		name   string
		want   bool
	}{
		{name: "installed", result: stubResult{out: "install ok installed"}, want: true},
		{name: "removed but not purged", result: stubResult{out: "deinstall ok config-files"}, want: false},
		{name: "dpkg-query errors (not present at all)", result: stubResult{err: errors.New("no such package")}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := stubRunner(t, map[string]stubResult{"dpkg-query": tc.result})
			if got := packageInstalled(context.Background(), runner, "some-package"); got != tc.want {
				t.Errorf("packageInstalled = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunCombinesBothFindings(t *testing.T) {
	// fail2banJailLocalPath and unattendedUpgradesConfigPath are fixed
	// absolute host paths that don't exist on a dev/CI box, so both
	// finding constructors deterministically fall through to their
	// "active/installed but unconfigured" branch below.
	runner := stubRunner(t, map[string]stubResult{
		"systemctl":  {err: nil},
		"dpkg-query": {out: "install ok installed"},
	})

	findings, err := run(context.Background(), runner)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}
	if findings[0].TestID != "THEMIS-FAIL2BAN" {
		t.Errorf("findings[0].TestID = %q, want THEMIS-FAIL2BAN", findings[0].TestID)
	}
	if findings[1].TestID != "THEMIS-UNATTENDED-UPGRADES" {
		t.Errorf("findings[1].TestID = %q, want THEMIS-UNATTENDED-UPGRADES", findings[1].TestID)
	}
}

func TestRunInactiveAndNotInstalled(t *testing.T) {
	boom := errors.New("boom")
	runner := stubRunner(t, map[string]stubResult{
		"systemctl":  {err: boom},
		"dpkg-query": {err: boom},
	})

	findings, err := run(context.Background(), runner)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}
	if findings[0].Description != "fail2ban is not installed or not active" {
		t.Errorf("findings[0].Description = %q", findings[0].Description)
	}
	if findings[1].Description != "unattended-upgrades is not installed" {
		t.Errorf("findings[1].Description = %q", findings[1].Description)
	}
}

func TestSourceRun(t *testing.T) {
	// Exercises Source.Run's exported entry point end to end against
	// whatever trusted binaries actually exist on the host running the
	// test. On a box without systemctl/dpkg-query (this repo's dev/CI
	// environment included) runCmd/packageInstalled fail to resolve them
	// and fall back to "inactive"/"not installed", which is itself a
	// valid, error-free outcome — the point of this test is that Run
	// completes without erroring, not a specific finding set.
	if _, err := (Source{}).Run(context.Background()); err != nil {
		t.Fatalf("Source.Run: %v", err)
	}
}
