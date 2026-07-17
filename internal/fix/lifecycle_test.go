package fix

import (
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
	active := false
	installed := false
	run := func(name string, args ...string) error {
		joined := name + " " + strings.Join(args, " ")
		if strings.Contains(joined, "--force enable") {
			active = true
		}
		return nil
	}
	outRun := func(name string, args ...string) (string, error) {
		if active {
			return "Status: active\nDefault: deny (incoming), allow (outgoing)\n", nil
		}
		return "", errors.New("ufw not running")
	}
	f := firewallDefaultDenyFixWith(run, outRun, func(string) bool { return installed })

	if ok, err := f.Check(); err != nil || ok {
		t.Fatalf("pre-apply Check = %v, %v; want false, nil", ok, err)
	}

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if ok, err := f.Check(); err != nil || !ok {
		t.Fatalf("post-apply Check = %v, %v; want true, nil", ok, err)
	}

	// Not installed before → revert removes ufw and returns nil.
	if revErr := f.Revert(data); revErr != nil {
		t.Fatalf("Revert: %v", revErr)
	}
}

func TestFirewallApplyInstallFailure(t *testing.T) {
	run := func(name string, args ...string) error {
		if strings.Contains(name+" "+strings.Join(args, " "), "apt-get install") {
			return errors.New("install failed")
		}
		return nil
	}
	outRun := func(string, ...string) (string, error) { return "", nil }
	f := firewallDefaultDenyFixWith(run, outRun, func(string) bool { return false })
	if _, err := f.Apply(); err == nil {
		t.Fatal("expected Apply to fail when install fails")
	}
}

func TestFirewallRevertRestoresPriorPolicy(t *testing.T) {
	var restored string
	run := func(name string, args ...string) error {
		joined := name + " " + strings.Join(args, " ")
		if strings.HasPrefix(joined, "ufw default ") {
			restored = joined
		}
		return nil
	}
	// Was installed and active, prior policy allow → revert restores it, keeps active.
	outRun := func(string, ...string) (string, error) {
		return "Status: active\nDefault: allow (incoming), allow (outgoing)\n", nil
	}
	f := firewallDefaultDenyFixWith(run, outRun, func(string) bool { return true })
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
}
