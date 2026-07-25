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

func TestSysctlFixAtRevertWarnDetectsPostApplyDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	f := sysctlFixAt(path, func() error { return nil })

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if f.RevertWarn == nil {
		t.Fatal("expected RevertWarn to be set")
	}
	if _, detected, warnErr := f.RevertWarn(revertData); warnErr != nil || detected {
		t.Fatalf("RevertWarn immediately after Apply = (detected=%v, err=%v), want (false, nil)", detected, warnErr)
	}

	// Simulate an admin appending a legitimate line after apply.
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	drifted := string(current) + "net.ipv4.conf.all.log_martians = 1\n"
	if err = os.WriteFile(path, []byte(drifted), 0o644); err != nil {
		t.Fatalf("simulating drift: %v", err)
	}

	msg, detected, err := f.RevertWarn(revertData)
	if err != nil {
		t.Fatalf("RevertWarn: %v", err)
	}
	if !detected {
		t.Fatal("expected RevertWarn to detect drift after a post-apply edit")
	}
	if msg == "" {
		t.Fatal("expected a non-empty warning message")
	}

	if err := f.Revert(revertData); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Revert should have discarded the drift once called — RevertWarn only informs, it doesn't block")
	}
}
