package fix

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHDisableDirectiveFixAtLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("Port 22\nPermitRootLogin yes\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	noopReload := func() error { return nil }
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, noopReload, "PermitRootLogin")

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if satisfied {
		t.Fatal("expected Check to report unsatisfied before Apply")
	}

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	satisfied, err = f.Check()
	if err != nil {
		t.Fatalf("Check after Apply: %v", err)
	}
	if !satisfied {
		t.Fatal("expected Check to report satisfied after Apply")
	}

	if revertErr := f.Revert(revertData); revertErr != nil {
		t.Fatalf("Revert: %v", revertErr)
	}
	satisfied, err = f.Check()
	if err != nil {
		t.Fatalf("Check after Revert: %v", err)
	}
	if satisfied {
		t.Fatal("expected Check to report unsatisfied after Revert restored original")
	}
}

// TestSSHDisableDirectiveFixAtIgnoresNarrowerMatchException reproduces
// issue #15 scenario A: a genuinely wide-open global PermitRootLogin yes
// with a Match block that only tightens it for one subnet. Check must
// report unsatisfied (not mask the global exposure), and Apply must fix
// the global line while leaving the Match block's already-correct line
// alone.
func TestSSHDisableDirectiveFixAtIgnoresNarrowerMatchException(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	content := "PasswordAuthentication no\nPermitRootLogin yes\nMatch Address 10.0.0.0/8\n    PermitRootLogin no\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if satisfied {
		t.Fatal("expected Check to report unsatisfied — the global default is still wide open")
	}

	if _, applyErr := f.Apply(); applyErr != nil {
		t.Fatalf("Apply: %v", applyErr)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading updated config: %v", err)
	}
	if !strings.Contains(string(updated), "Match Address 10.0.0.0/8\n    PermitRootLogin no") {
		t.Fatalf("expected Match block to survive untouched, got:\n%s", updated)
	}
	satisfied, err = f.Check()
	if err != nil {
		t.Fatalf("Check after Apply: %v", err)
	}
	if !satisfied {
		t.Fatal("expected Check to report satisfied after Apply fixed the global default")
	}
}

// TestSSHDisableDirectiveFixAtDoesNotDestroyMatchException reproduces
// issue #15 scenario B: global already hardened, with a deliberate
// Match-scoped break-glass exception. Check must report satisfied (no
// false-positive "needs fixing"), so Apply is never invoked and the
// operator's override is never at risk of being silently commented out.
func TestSSHDisableDirectiveFixAtDoesNotDestroyMatchException(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	content := "PasswordAuthentication no\nPermitRootLogin no\nMatch User admin\n    PermitRootLogin yes\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !satisfied {
		t.Fatal("expected Check to report satisfied — the global default is already hardened")
	}

	// Belt-and-suspenders: even if Apply were invoked, it must not touch
	// the admin override.
	if _, applyErr := f.Apply(); applyErr != nil {
		t.Fatalf("Apply: %v", applyErr)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if !strings.Contains(string(updated), "Match User admin\n    PermitRootLogin yes") {
		t.Fatalf("expected admin's Match-scoped override to survive, got:\n%s", updated)
	}
}

// TestSSHDisableDirectiveFixAtRevertWarnDetectsPostApplyDrift reproduces
// issue #16's SSH-7408-ROOTLOGIN repro: an admin appends a legitimate Match
// block after Apply ran. RevertWarn must flag the drift so Revert doesn't
// silently discard it.
func TestSSHDisableDirectiveFixAtRevertWarnDetectsPostApplyDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("PermitRootLogin yes\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	matchBlock := "\nMatch User deploy\n    PermitRootLogin yes\n"
	if writeErr := os.WriteFile(path, append(current, []byte(matchBlock)...), 0o600); writeErr != nil {
		t.Fatalf("simulating admin edit: %v", writeErr)
	}

	if f.RevertWarn == nil {
		t.Fatal("expected RevertWarn to be set")
	}
	msg, detected, err := f.RevertWarn(revertData)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if !detected {
		t.Fatal("expected RevertWarn to detect the post-apply Match block")
	}
	if msg == "" {
		t.Fatal("expected a non-empty warning message")
	}
}

func TestSSHDisableDirectiveFixAtRevertWarnNoDriftWhenUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("PermitRootLogin yes\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	_, detected, err := f.RevertWarn(revertData)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if detected {
		t.Fatal("expected no drift when the file is untouched since Apply")
	}
}

func TestSSHDisableDirectiveFixAtWarnsWhenMatchBlockRedefinesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	content := "PermitRootLogin yes\nMatch Address 10.0.0.0/8\n    PermitRootLogin no\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	if f.Warn == nil {
		t.Fatal("expected Warn to be set")
	}
	msg, detected, err := f.Warn()
	if err != nil {
		t.Fatalf("Warn: %v", err)
	}
	if !detected {
		t.Fatal("expected Warn to detect the Match-scoped redefinition")
	}
	if msg == "" {
		t.Fatal("expected a non-empty warning message")
	}
}

func TestSSHDisableDirectiveFixAtNoWarnWithoutMatchBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("PermitRootLogin yes\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	_, detected, err := f.Warn()
	if err != nil {
		t.Fatalf("Warn: %v", err)
	}
	if detected {
		t.Fatal("expected Warn not to fire when there is no Match block")
	}
}

func TestSSHDisableDirectiveFixAtMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	f := sshDisableDirectiveFixAt("TEST-ID", "test fix", path, func() error { return nil }, "PermitRootLogin")

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check on missing file: %v", err)
	}
	if satisfied {
		t.Fatal("expected unsatisfied when config file doesn't exist")
	}

	if _, err := f.Apply(); err != nil {
		t.Fatalf("Apply should create the file: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist after Apply: %v", err)
	}
}

func TestAuthorizedKeysExist(t *testing.T) {
	t.Run("no homes have a .ssh dir", func(t *testing.T) {
		homes := []string{t.TempDir(), t.TempDir()}
		ok, err := authorizedKeysExist(homes)
		if err != nil {
			t.Fatalf("authorizedKeysExist: %v", err)
		}
		if ok {
			t.Fatal("expected false when no home has authorized_keys")
		}
	})

	t.Run("authorized_keys file exists but is empty", func(t *testing.T) {
		home := t.TempDir()
		sshDir := filepath.Join(home, ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			t.Fatalf("mkdir .ssh: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("   \n"), 0o600); err != nil {
			t.Fatalf("writing empty authorized_keys: %v", err)
		}
		ok, err := authorizedKeysExist([]string{home})
		if err != nil {
			t.Fatalf("authorizedKeysExist: %v", err)
		}
		if ok {
			t.Fatal("expected false when authorized_keys is blank")
		}
	})

	t.Run("one of several homes has a populated authorized_keys", func(t *testing.T) {
		emptyHome := t.TempDir()
		keyedHome := t.TempDir()
		sshDir := filepath.Join(keyedHome, ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			t.Fatalf("mkdir .ssh: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-ed25519 AAAA... user@host\n"), 0o600); err != nil {
			t.Fatalf("writing authorized_keys: %v", err)
		}
		ok, err := authorizedKeysExist([]string{emptyHome, keyedHome})
		if err != nil {
			t.Fatalf("authorizedKeysExist: %v", err)
		}
		if !ok {
			t.Fatal("expected true when a home has a non-empty authorized_keys")
		}
	})
}

func TestHomeDirectoriesFrom(t *testing.T) {
	passwd := filepath.Join(t.TempDir(), "passwd")
	content := "root:x:0:0:root:/root:/bin/bash\n" +
		"zeus:x:1000:1000:zeus:/home/zeus:/bin/bash\n" +
		"daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin\n" +
		"malformed-line-without-enough-fields\n" +
		"nohome:x:2000:2000:nohome::/bin/bash\n"
	if err := os.WriteFile(passwd, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fake passwd: %v", err)
	}

	homes, err := homeDirectoriesFrom(passwd)
	if err != nil {
		t.Fatalf("homeDirectoriesFrom: %v", err)
	}

	want := []string{"/root", "/root", "/home/zeus", "/usr/sbin"}
	if len(homes) != len(want) {
		t.Fatalf("got %v, want %v", homes, want)
	}
	for i, h := range want {
		if homes[i] != h {
			t.Fatalf("got %v, want %v", homes, want)
		}
	}
}

func TestHomeDirectoriesFromMissingFile(t *testing.T) {
	_, err := homeDirectoriesFrom(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error when passwd file is missing")
	}
}

func TestSSHPasswordAuthFixWithRefusesWithoutAuthorizedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("Port 22\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	noHomes := func() ([]string, error) { return []string{t.TempDir()}, nil }
	f := sshPasswordAuthFixWith(path, func() error { return nil }, noHomes)

	if _, err := f.Apply(); err == nil {
		t.Fatal("expected Apply to refuse when no authorized_keys exist")
	}

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if satisfied {
		t.Fatal("expected sshd_config to be untouched after refused Apply")
	}
}

func TestSSHPasswordAuthFixWithAppliesWhenAuthorizedKeysPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("Port 22\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-ed25519 AAAA... user@host\n"), 0o600); err != nil {
		t.Fatalf("writing authorized_keys: %v", err)
	}
	withHome := func() ([]string, error) { return []string{home}, nil }
	f := sshPasswordAuthFixWith(path, func() error { return nil }, withHome)

	if _, err := f.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !satisfied {
		t.Fatal("expected sshd_config to be updated after Apply")
	}
}

func TestSSHPasswordAuthFixWithPropagatesHomeDirsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("Port 22\n"), 0o600); err != nil {
		t.Fatalf("seeding sshd_config: %v", err)
	}
	boom := errors.New("boom")
	failingHomes := func() ([]string, error) { return nil, boom }
	f := sshPasswordAuthFixWith(path, func() error { return nil }, failingHomes)

	if _, err := f.Apply(); !errors.Is(err, boom) {
		t.Fatalf("expected Apply to propagate homeDirs error, got %v", err)
	}
}
