package fix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSysctlFixAtLifecycleWhenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	noopReload := func() error { return nil }
	f := sysctlFixAt(path, noopReload)

	satisfied, err := f.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if satisfied {
		t.Fatal("expected unsatisfied before Apply")
	}

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if revertData != nil {
		t.Fatalf("expected nil revert sentinel for a file that didn't exist, got %v", revertData)
	}

	satisfied, err = f.Check()
	if err != nil {
		t.Fatalf("Check after Apply: %v", err)
	}
	if !satisfied {
		t.Fatal("expected satisfied after Apply")
	}

	if err := f.Revert(revertData); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed after Revert, stat err = %v", err)
	}
}

// TestSysctlFixAtRevertWarnDetectsPostApplyDrift reproduces issue #16's
// KRNL-6000 repro: an admin appends a legitimate line to the themis-managed
// drop-in after Apply ran. RevertWarn must flag the drift so Revert doesn't
// silently discard it.
func TestSysctlFixAtRevertWarnDetectsPostApplyDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	f := sysctlFixAt(path, func() error { return nil })

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	drifted := "net.ipv4.conf.all.log_martians = 1\n"
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	if writeErr := os.WriteFile(path, append(current, []byte(drifted)...), 0o644); writeErr != nil {
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
		t.Fatal("expected RevertWarn to detect the post-apply edit")
	}
	if msg == "" {
		t.Fatal("expected a non-empty warning message")
	}
}

func TestSysctlFixAtRevertWarnNoDriftWhenUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	f := sysctlFixAt(path, func() error { return nil })

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

func TestSysctlFixAtRevertWarnNoDriftWhenFileRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	f := sysctlFixAt(path, func() error { return nil })

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if removeErr := os.Remove(path); removeErr != nil {
		t.Fatalf("removing file: %v", removeErr)
	}

	_, detected, err := f.RevertWarn(revertData)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if detected {
		t.Fatal("expected no drift when the file is simply gone — nothing left to discard")
	}
}

func TestSysctlFixAtLifecycleWhenFilePreexisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	original := "# custom pre-existing content\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	f := sysctlFixAt(path, func() error { return nil })

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if string(revertData) != original {
		t.Fatalf("expected revert data to be original content %q, got %q", original, revertData)
	}

	if revertErr := f.Revert(revertData); revertErr != nil {
		t.Fatalf("Revert: %v", revertErr)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading reverted file: %v", err)
	}
	if string(got) != original {
		t.Fatalf("expected reverted content %q, got %q", original, got)
	}
}
