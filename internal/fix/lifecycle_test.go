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
	enabled   bool
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
		r.enabled = true
	case strings.Contains(joined, "disable --now"):
		r.active = false
		r.enabled = false
	case joined == "systemctl enable fail2ban":
		r.enabled = true
	case joined == "systemctl disable fail2ban":
		r.enabled = false
	case joined == "systemctl stop fail2ban":
		r.active = false
	case joined == "systemctl restart fail2ban":
		r.active = true
	}
	return nil
}

func (r *fakeRunner) isActive(name string, args ...string) error {
	joined := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, joined)
	if strings.Contains(joined, "is-active") && !r.active {
		return errors.New("inactive")
	}
	if strings.Contains(joined, "is-enabled") && !r.enabled {
		return errors.New("disabled")
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

// TestFail2banRevertStopsAndDisablesWhenInactiveBefore reproduces issue #14:
// fail2ban was installed but inactive/disabled before Apply (the common
// Debian case). Revert must leave it stopped and disabled again, not just
// restart it into an active+enabled state it never had before Apply.
func TestFail2banRevertStopsAndDisablesWhenInactiveBefore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	r := &fakeRunner{installed: true} // installed, but inactive/disabled before Apply
	f := fail2banFixWith(path, r.isActive, func(string) bool { return r.installed })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !r.active || !r.enabled {
		t.Fatalf("expected Apply to leave fail2ban active+enabled, got active=%v enabled=%v", r.active, r.enabled)
	}

	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	if r.active {
		t.Error("expected Revert to stop fail2ban that was inactive before Apply")
	}
	if r.enabled {
		t.Error("expected Revert to disable fail2ban that was disabled before Apply")
	}
}

// TestFail2banRevertRestartsWhenActiveBefore covers the complementary case:
// fail2ban was already active+enabled before Apply, so Revert must restore
// the config and leave it running rather than stopping/disabling it.
func TestFail2banRevertRestartsWhenActiveBefore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	r := &fakeRunner{installed: true, active: true, enabled: true}
	f := fail2banFixWith(path, r.isActive, func(string) bool { return r.installed })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
	if !r.active {
		t.Error("expected Revert to leave fail2ban active when it was active before Apply")
	}
	if !r.enabled {
		t.Error("expected Revert to leave fail2ban enabled when it was enabled before Apply")
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
