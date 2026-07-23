package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSysctlKernel simulates the live kernel's sysctl values in memory, so
// tests can assert Revert restores them explicitly rather than just
// removing/restoring the drop-in file.
type fakeSysctlKernel struct {
	values map[string]string
}

func (k *fakeSysctlKernel) outRun(name string, args ...string) (string, error) {
	if name != "sysctl" || len(args) != 2 || args[0] != "-n" {
		return "", nil
	}
	return k.values[args[1]], nil
}

func (k *fakeSysctlKernel) run(name string, args ...string) error {
	if name != "sysctl" || len(args) != 2 || args[0] != "-w" {
		return nil
	}
	key, val, ok := strings.Cut(args[1], "=")
	if !ok {
		return nil
	}
	k.values[key] = val
	return nil
}

func newFakeSysctlKernel() *fakeSysctlKernel {
	values := make(map[string]string, len(sysctlHardeningSettings))
	for _, key := range sysctlKeys() {
		values[key] = "unset" // distinct sentinel per key before any operator/apply write
	}
	return &fakeSysctlKernel{values: values}
}

// reloadFrom simulates `sysctl --system` re-reading path and applying any
// "key = value" lines it finds, standing in for the real reload command in
// tests that need Apply's file write to actually land in the fake kernel.
func (k *fakeSysctlKernel) reloadFrom(path string) error {
	content, existed, err := ReadFileOrEmpty(path)
	if err != nil {
		return err
	}
	if !existed {
		return nil
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		key, val, ok := strings.Cut(line, " = ")
		if !ok {
			continue
		}
		k.values[key] = val
	}
	return nil
}

func TestSysctlFixAtLifecycleWhenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	kernel := newFakeSysctlKernel()
	noopReload := func() error { return nil }
	f := sysctlFixAt(path, kernel.outRun, kernel.run, noopReload)

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
	kernel := newFakeSysctlKernel()
	f := sysctlFixAt(path, kernel.outRun, kernel.run, func() error { return nil })

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
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

// TestSysctlFixAtRevertRestoresLiveKernelValues is the regression test for
// issue #20: apply -> rollback on a box with no pre-existing drop-in file
// must restore each of the 8 keys to their prior live value, not leave them
// at themis's hardened setting just because the file is gone. Reproduces
// the repro from the issue: net.ipv4.tcp_syncookies pre-set to "0" by an
// operator, with no file ever having declared it.
func TestSysctlFixAtRevertRestoresLiveKernelValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-themis-hardening.conf")
	kernel := newFakeSysctlKernel()
	kernel.values["net.ipv4.tcp_syncookies"] = "0" // operator's pre-existing config, no file behind it

	f := sysctlFixAt(path, kernel.outRun, kernel.run, func() error { return kernel.reloadFrom(path) })

	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := kernel.values["net.ipv4.tcp_syncookies"]; got != "1" {
		t.Fatalf("expected Apply to harden tcp_syncookies to 1, got %q", got)
	}

	if err := f.Revert(revertData); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected drop-in file removed after Revert, stat err = %v", err)
	}
	if got := kernel.values["net.ipv4.tcp_syncookies"]; got != "0" {
		t.Fatalf("expected Revert to restore live tcp_syncookies to operator's original 0, got %q (bug: file removed but live value never restored)", got)
	}
	for _, key := range sysctlKeys() {
		if key == "net.ipv4.tcp_syncookies" {
			continue
		}
		if got := kernel.values[key]; got != "unset" {
			t.Fatalf("expected %s restored to its prior live value %q, got %q", key, "unset", got)
		}
	}
}
