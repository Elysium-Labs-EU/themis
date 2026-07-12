package fix

import (
	"os"
	"path/filepath"
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
