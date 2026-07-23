package fix

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner records invocations and lets a test flip service state or force
// specific commands to fail, so Apply/Revert run without touching the host.
type fakeRunner struct {
	failOn    string // substring of "name args"; matching calls return an error
	calls     []string
	active    bool
	installed bool
}

func (r *fakeRunner) run(name string, args ...string) error {
	joined := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, joined)
	if r.failOn != "" && strings.Contains(joined, r.failOn) {
		return errors.New("forced failure: " + joined)
	}
	switch {
	case strings.Contains(joined, "enable --now"):
		r.active = true
	case strings.Contains(joined, "disable --now"):
		r.active = false
	}
	return nil
}

func (r *fakeRunner) isActive(name string, args ...string) error {
	joined := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, joined)
	if strings.Contains(joined, "is-active") && !r.active {
		return errors.New("inactive")
	}
	return r.run(name, args...)
}

func TestFail2banFixLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	r := &fakeRunner{}
	f := fail2banFixWith(path, r.isActive, func(string) bool { return r.installed })

	if ok, err := f.Check(); err != nil || ok {
		t.Fatalf("pre-apply Check = %v, %v; want false, nil", ok, err)
	}

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected jail.local written: %v", statErr)
	}

	if ok, err := f.Check(); err != nil || !ok {
		t.Fatalf("post-apply Check = %v, %v; want true, nil", ok, err)
	}

	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	// Config was absent before Apply → Revert removes it.
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected jail.local removed on revert, stat err = %v", statErr)
	}
}

func TestFail2banApplyInstallFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	r := &fakeRunner{failOn: "apt-get install"}
	f := fail2banFixWith(path, r.isActive, func(string) bool { return false })
	if _, err := f.Apply(); err == nil {
		t.Fatal("expected Apply to fail when install fails")
	}
}

func TestFail2banRevertRestoresExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	prior := "[sshd]\nenabled = false\n"
	if err := os.WriteFile(path, []byte(prior), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := &fakeRunner{installed: true}
	f := fail2banFixWith(path, r.isActive, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after revert: %v", err)
	}
	if string(got) != prior {
		t.Fatalf("revert restored %q, want %q", got, prior)
	}
}

// TestFail2banFixRevertWarnDetectsPostApplyDrift covers issue #16: an
// operator hand-edits jail.local after Apply ran (e.g. adding another jail
// section). RevertWarn must flag the drift so Revert doesn't silently
// discard it.
func TestFail2banFixRevertWarnDetectsPostApplyDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	r := &fakeRunner{installed: true}
	f := fail2banFixWith(path, r.isActive, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	extra := "\n[nginx-http-auth]\nenabled = true\n"
	if writeErr := os.WriteFile(path, append(current, []byte(extra)...), 0o644); writeErr != nil {
		t.Fatalf("simulating admin edit: %v", writeErr)
	}

	if f.RevertWarn == nil {
		t.Fatal("expected RevertWarn to be set")
	}
	msg, detected, err := f.RevertWarn(data)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if !detected {
		t.Fatal("expected RevertWarn to detect the post-apply edit")
	}
	if msg == "" {
		t.Fatal("expected a non-empty warning message")
	}
}

func TestFail2banFixRevertWarnNoDriftWhenUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	r := &fakeRunner{installed: true}
	f := fail2banFixWith(path, r.isActive, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	_, detected, err := f.RevertWarn(data)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if detected {
		t.Fatal("expected no drift when the file is untouched since Apply")
	}
}

func TestAutoUpdatesFixLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20auto-upgrades")
	r := &fakeRunner{}
	installed := false
	f := autoUpdatesFixWith(path, r.run, func(string) bool { return installed })

	if ok, err := f.Check(); err != nil || ok {
		t.Fatalf("pre-apply Check = %v, %v; want false, nil", ok, err)
	}

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	installed = true // Apply would have installed it.
	if ok, err := f.Check(); err != nil || !ok {
		t.Fatalf("post-apply Check = %v, %v; want true, nil", ok, err)
	}

	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected config removed on revert, stat err = %v", statErr)
	}
}

func TestAutoUpdatesCheckNotInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20auto-upgrades")
	f := autoUpdatesFixWith(path, (&fakeRunner{}).run, func(string) bool { return false })
	if ok, err := f.Check(); err != nil || ok {
		t.Fatalf("Check with package absent = %v, %v; want false, nil", ok, err)
	}
}

// TestAutoUpdatesFixRevertWarnDetectsPostApplyDrift covers issue #16: an
// operator hand-edits the 20auto-upgrades config after Apply ran.
// RevertWarn must flag the drift so Revert doesn't silently discard it.
func TestAutoUpdatesFixRevertWarnDetectsPostApplyDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20auto-upgrades")
	f := autoUpdatesFixWith(path, (&fakeRunner{}).run, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	extra := `APT::Periodic::AutocleanInterval "7";` + "\n"
	if writeErr := os.WriteFile(path, append(current, []byte(extra)...), 0o644); writeErr != nil {
		t.Fatalf("simulating admin edit: %v", writeErr)
	}

	if f.RevertWarn == nil {
		t.Fatal("expected RevertWarn to be set")
	}
	msg, detected, err := f.RevertWarn(data)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if !detected {
		t.Fatal("expected RevertWarn to detect the post-apply edit")
	}
	if msg == "" {
		t.Fatal("expected a non-empty warning message")
	}
}

func TestAutoUpdatesFixRevertWarnNoDriftWhenUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20auto-upgrades")
	f := autoUpdatesFixWith(path, (&fakeRunner{}).run, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	_, detected, err := f.RevertWarn(data)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if detected {
		t.Fatal("expected no drift when the file is untouched since Apply")
	}
}

func TestAutoUpdatesRevertRestoresExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20auto-upgrades")
	prior := `APT::Periodic::Unattended-Upgrade "0";` + "\n"
	if err := os.WriteFile(path, []byte(prior), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	f := autoUpdatesFixWith(path, (&fakeRunner{}).run, func(string) bool { return true })
	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after revert: %v", err)
	}
	if string(got) != prior {
		t.Fatalf("revert restored %q, want %q", got, prior)
	}
}

func TestFirewallFixLifecycle(t *testing.T) {
	sshdPath := filepath.Join(t.TempDir(), "sshd_config") // absent → defaults to port 22
	active := false
	installed := false
	sshAllowed := false
	run := func(name string, args ...string) error {
		joined := name + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--force enable"):
			active = true
		case strings.Contains(joined, "allow 22/tcp"):
			sshAllowed = true
		}
		return nil
	}
	outRun := func(name string, args ...string) (string, error) {
		if !active {
			return "", errors.New("ufw not running")
		}
		out := "Status: active\nDefault: deny (incoming), allow (outgoing)\n"
		if sshAllowed {
			out += "22/tcp                     ALLOW IN    Anywhere\n"
		}
		return out, nil
	}
	f := firewallDefaultDenyFixWith(sshdPath, run, outRun, func(string) bool { return installed })

	if ok, err := f.Check(); err != nil || ok {
		t.Fatalf("pre-apply Check = %v, %v; want false, nil", ok, err)
	}

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !sshAllowed {
		t.Fatal("expected Apply to allow port 22/tcp before enabling ufw")
	}

	if ok, err := f.Check(); err != nil || !ok {
		t.Fatalf("post-apply Check = %v, %v; want true, nil", ok, err)
	}

	// Not installed before → revert removes ufw and returns nil.
	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
}

func TestFirewallApplyAllowsConfiguredSSHDPort(t *testing.T) {
	sshdPath := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(sshdPath, []byte("Port 2222\n"), 0o644); err != nil {
		t.Fatalf("seed sshd_config: %v", err)
	}
	var allowed []string
	run := func(name string, args ...string) error {
		joined := name + " " + strings.Join(args, " ")
		if strings.HasPrefix(joined, "ufw allow ") {
			allowed = append(allowed, joined)
		}
		return nil
	}
	outRun := func(string, ...string) (string, error) { return "", errors.New("ufw not running") }
	f := firewallDefaultDenyFixWith(sshdPath, run, outRun, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(allowed) != 1 || allowed[0] != "ufw allow 2222/tcp" {
		t.Fatalf("expected exactly one 'ufw allow 2222/tcp', got %v", allowed)
	}

	var state firewallState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal revert state: %v", err)
	}
	if len(state.AllowedPorts) != 1 || state.AllowedPorts[0] != "2222" {
		t.Fatalf("expected AllowedPorts = [2222], got %v", state.AllowedPorts)
	}
}

func TestFirewallApplySkipsPortsAlreadyAllowed(t *testing.T) {
	sshdPath := filepath.Join(t.TempDir(), "sshd_config")
	var allowCalls []string
	run := func(name string, args ...string) error {
		joined := name + " " + strings.Join(args, " ")
		if strings.HasPrefix(joined, "ufw allow ") {
			allowCalls = append(allowCalls, joined)
		}
		return nil
	}
	// Prior state already has an OpenSSH allow rule.
	outRun := func(string, ...string) (string, error) {
		return "Status: inactive\nOpenSSH                    ALLOW IN    Anywhere\n", nil
	}
	f := firewallDefaultDenyFixWith(sshdPath, run, outRun, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(allowCalls) != 0 {
		t.Fatalf("expected no new allow rule when OpenSSH already allowed, got %v", allowCalls)
	}
	var state firewallState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal revert state: %v", err)
	}
	if len(state.AllowedPorts) != 0 {
		t.Fatalf("expected no AllowedPorts recorded, got %v", state.AllowedPorts)
	}
}

func TestFirewallApplyInstallFailure(t *testing.T) {
	sshdPath := filepath.Join(t.TempDir(), "sshd_config")
	run := func(name string, args ...string) error {
		if strings.Contains(name+" "+strings.Join(args, " "), "apt-get install") {
			return errors.New("install failed")
		}
		return nil
	}
	outRun := func(string, ...string) (string, error) { return "", nil }
	f := firewallDefaultDenyFixWith(sshdPath, run, outRun, func(string) bool { return false })
	if _, err := f.Apply(); err == nil {
		t.Fatal("expected Apply to fail when install fails")
	}
}

func TestFirewallRevertRestoresPriorPolicy(t *testing.T) {
	sshdPath := filepath.Join(t.TempDir(), "sshd_config")
	var restored string
	var deleted []string
	run := func(name string, args ...string) error {
		joined := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "ufw default "):
			restored = joined
		case strings.HasPrefix(joined, "ufw delete allow "):
			deleted = append(deleted, joined)
		}
		return nil
	}
	// Was installed and active, prior policy allow, no existing SSH rule →
	// Apply adds one, revert restores prior policy, keeps active, and
	// removes exactly the rule Apply added.
	outRun := func(string, ...string) (string, error) {
		return "Status: active\nDefault: allow (incoming), allow (outgoing)\n", nil
	}
	f := firewallDefaultDenyFixWith(sshdPath, run, outRun, func(string) bool { return true })
	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	if !strings.Contains(restored, "allow incoming") {
		t.Fatalf("expected prior allow policy restored, got %q", restored)
	}
	if len(deleted) != 1 || deleted[0] != "ufw delete allow 22/tcp" {
		t.Fatalf("expected revert to delete exactly the SSH rule Apply added, got %v", deleted)
	}
}
