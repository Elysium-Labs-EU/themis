package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/config"
)

func TestApiCheckResultSerializesDriftSeparatelyFromFindings(t *testing.T) {
	result := apiCheckResult{
		Findings: []apiFinding{{TestID: "SSH-7408", Kind: "warning"}},
		Drift: []apiDrift{
			{TestID: "THEMIS-FAIL2BAN", Description: "fail2ban stopped running", Details: "applied 2026-01-01T00:00:00Z, no longer satisfied"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		`"drift":[{"test_id":"THEMIS-FAIL2BAN"`,
		`"details":"applied 2026-01-01T00:00:00Z, no longer satisfied"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}

	var round apiCheckResult
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("round-tripping: %v", err)
	}
	if len(round.Drift) != 1 || round.Drift[0].TestID != "THEMIS-FAIL2BAN" {
		t.Fatalf("round-tripped Drift = %+v", round.Drift)
	}
	if len(round.Findings) != 1 || round.Findings[0].TestID != "SSH-7408" {
		t.Fatalf("round-tripped Findings = %+v, expected drift not to have leaked in", round.Findings)
	}
}

// runAPICheck runs apiCheckCmd.RunE with fresh stdout/stderr buffers and
// restores them afterward, mirroring how cobra wires a command at runtime.
func runAPICheck(t *testing.T) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	apiCheckCmd.SetOut(stdout)
	apiCheckCmd.SetErr(stderr)
	apiCheckCmd.SetContext(context.Background())
	t.Cleanup(func() {
		apiCheckCmd.SetOut(nil)
		apiCheckCmd.SetErr(nil)
	})
	err = apiCheckCmd.RunE(apiCheckCmd, nil)
	return stdout, stderr, err
}

// TestApiCheckCmdRunEErrorsOnUnreadableConfig covers the loadOperatorConfig
// failure branch: a malformed THEMIS_CONFIG file makes RunE write a JSON
// error to stderr and return errAPICommandFailed rather than a raw error,
// so callers of `themis api check` get a stable error schema.
func TestApiCheckCmdRunEErrorsOnUnreadableConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sources: [this is not valid yaml"), 0o600); err != nil {
		t.Fatalf("writing broken config: %v", err)
	}
	t.Setenv(config.EnvVar, path)

	_, stderr, err := runAPICheck(t)
	if !errors.Is(err, errAPICommandFailed) {
		t.Fatalf("RunE error = %v, want errAPICommandFailed", err)
	}

	var payload map[string]string
	if jerr := json.Unmarshal(stderr.Bytes(), &payload); jerr != nil {
		t.Fatalf("stderr not valid JSON: %v\n%s", jerr, stderr.String())
	}
	if !strings.Contains(payload["error"], "loading operator config") {
		t.Errorf("error = %q, want it to mention loading operator config", payload["error"])
	}
}

// TestApiCheckCmdRunEErrorsWithoutLynisBinary covers the audit.Run failure
// branch: with the default config (lynis enabled) and no lynis binary on
// PATH, RunE must surface the same JSON error schema as the config-load
// failure above, not a bare Go error.
func TestApiCheckCmdRunEErrorsWithoutLynisBinary(t *testing.T) {
	if _, err := exec.LookPath("lynis"); err == nil {
		t.Skip("lynis is installed on this host; skipping the missing-binary path")
	}
	t.Setenv(config.EnvVar, filepath.Join(t.TempDir(), "unused-config.yaml"))

	_, stderr, err := runAPICheck(t)
	if !errors.Is(err, errAPICommandFailed) {
		t.Fatalf("RunE error = %v, want errAPICommandFailed", err)
	}

	var payload map[string]string
	if jerr := json.Unmarshal(stderr.Bytes(), &payload); jerr != nil {
		t.Fatalf("stderr not valid JSON: %v\n%s", jerr, stderr.String())
	}
	if payload["error"] == "" {
		t.Error("expected a non-empty error message")
	}
}

// TestApiCheckCmdRunESucceedsAndEmitsValidJSON drives the full success
// path — config load, source build/run, fix resolution, report assembly,
// and writeJSON — with lynis and osquery disabled so the run needs no
// external binaries or root privileges (only the themis-native source
// runs, which degrades gracefully when systemctl/dpkg-query are absent).
func TestApiCheckCmdRunESucceedsAndEmitsValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfgYAML := "sources:\n  lynis:\n    enabled: false\n  osquery:\n    enabled: false\n"
	if err := os.WriteFile(path, []byte(cfgYAML), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	t.Setenv(config.EnvVar, path)

	stdout, stderr, err := runAPICheck(t)
	if err != nil {
		t.Fatalf("RunE: %v\nstderr: %s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr output, got %q", stderr.String())
	}

	var result apiCheckResult
	if jerr := json.Unmarshal(stdout.Bytes(), &result); jerr != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", jerr, stdout.String())
	}
	// The themis-native source always reports on fail2ban and
	// unattended-upgrades unless both are actually configured correctly
	// on the host running this test, so on a bare CI box this is non-empty
	// and exercises the findings/fixes assembly loops below.
	for i := range result.Findings {
		if result.Findings[i].TestID == "" {
			t.Errorf("findings[%d] has an empty TestID", i)
		}
	}
}
