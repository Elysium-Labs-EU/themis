package lynis

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

func TestAuditRequiresRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test process is running as root; the requires-root guard can't be exercised")
	}
	_, err := Audit(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected an error when not running as root")
	}
	if _, ok := errors.AsType[*ui.UserError](err); !ok {
		t.Fatalf("error = %v (%T), want *ui.UserError", err, err)
	}
}

func TestReadReportParsesFindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lynis-report.dat")
	content := "suggestion[]=SSH-7408|Harden SSH config|-|Change PermitRootLogin|\n" +
		"warning[]=FIRE-4590|No firewall active|-|Enable ufw|\n" +
		"# unrelated report line, ignored\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seeding report: %v", err)
	}

	findings, err := readReport(path)
	if err != nil {
		t.Fatalf("readReport: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}
	if findings[0].TestID != "SSH-7408" || findings[0].Kind != "suggestion" {
		t.Errorf("finding[0] = %+v", findings[0])
	}
	if findings[1].TestID != "FIRE-4590" || findings[1].Kind != "warning" {
		t.Errorf("finding[1] = %+v", findings[1])
	}
}

func TestReadReportMissingFile(t *testing.T) {
	if _, err := readReport(filepath.Join(t.TempDir(), "does-not-exist.dat")); err == nil {
		t.Fatal("expected an error for a missing report file")
	}
}

func TestReadReportRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-report.dat")
	if err := os.WriteFile(target, []byte("suggestion[]=SSH-7408|x|-|-|\n"), 0o644); err != nil {
		t.Fatalf("seeding report: %v", err)
	}
	link := filepath.Join(dir, "lynis-report.dat")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if _, err := readReport(link); err == nil {
		t.Fatal("expected readReport to reject a symlinked report path")
	}
}

func TestReadReportRejectsGroupOrOtherWritableMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lynis-report.dat")
	if err := os.WriteFile(path, []byte("suggestion[]=SSH-7408|x|-|-|\n"), 0o644); err != nil {
		t.Fatalf("seeding report: %v", err)
	}
	if err := os.Chmod(path, 0o646); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := readReport(path); err == nil {
		t.Fatal("expected readReport to reject a group/other-writable report file")
	}
}

func TestVerifyReportOwnerAndModeRejectsWrongUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lynis-report.dat")
	if err := os.WriteFile(path, []byte("suggestion[]=SSH-7408|x|-|-|\n"), 0o644); err != nil {
		t.Fatalf("seeding report: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if err := verifyReportOwnerAndMode(info, os.Geteuid()+1); err == nil {
		t.Fatal("expected verifyReportOwnerAndMode to reject a UID mismatch")
	}
	if err := verifyReportOwnerAndMode(info, os.Geteuid()); err != nil {
		t.Errorf("verifyReportOwnerAndMode with the real UID: %v", err)
	}
}

func TestLynisArgsDefaultIsFullAudit(t *testing.T) {
	got := lynisArgs(Options{})
	want := []string{"audit", "system", "--quiet"}
	if !equalArgs(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLynisArgsQuickAddsFlag(t *testing.T) {
	got := lynisArgs(Options{Quick: true})
	want := []string{"audit", "system", "--quiet", "--quick"}
	if !equalArgs(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPriorityWrapNoToolsFound(t *testing.T) {
	resolve := func(string) (string, error) { return "", exec.ErrNotFound }

	bin, args := priorityWrap(resolve, "/usr/sbin/lynis", []string{"audit", "system"})
	if bin != "/usr/sbin/lynis" {
		t.Errorf("bin = %q, want unchanged", bin)
	}
	if !equalArgs(args, []string{"audit", "system"}) {
		t.Errorf("args = %v, want unchanged", args)
	}
}

func TestPriorityWrapNiceOnly(t *testing.T) {
	resolve := func(name string) (string, error) {
		if name == "nice" {
			return "/usr/bin/nice", nil
		}
		return "", exec.ErrNotFound
	}

	bin, args := priorityWrap(resolve, "/usr/sbin/lynis", []string{"audit", "system"})
	if bin != "/usr/bin/nice" {
		t.Errorf("bin = %q, want /usr/bin/nice", bin)
	}
	want := []string{"-n", "19", "/usr/sbin/lynis", "audit", "system"}
	if !equalArgs(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestPriorityWrapNiceAndIonice(t *testing.T) {
	resolve := func(name string) (string, error) {
		switch name {
		case "nice":
			return "/usr/bin/nice", nil
		case "ionice":
			return "/usr/bin/ionice", nil
		}
		return "", exec.ErrNotFound
	}

	bin, args := priorityWrap(resolve, "/usr/sbin/lynis", []string{"audit", "system"})
	if bin != "/usr/bin/ionice" {
		t.Errorf("bin = %q, want /usr/bin/ionice", bin)
	}
	want := []string{"-c3", "/usr/bin/nice", "-n", "19", "/usr/sbin/lynis", "audit", "system"}
	if !equalArgs(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestLynisNotFoundErrorHintIsPlatformNeutral(t *testing.T) {
	trustedDirs := []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin", "/usr/local/sbin", "/usr/local/bin"}
	got := lynisNotFoundError(trustedDirs)

	if got.Err.Error() != "lynis not found" {
		t.Errorf("Err = %q, want %q", got.Err.Error(), "lynis not found")
	}
	if strings.Contains(strings.ToLower(got.Hint), "apt") {
		t.Errorf("Hint = %q, want no Debian-specific package manager mentioned", got.Hint)
	}
	for _, dir := range trustedDirs {
		if !strings.Contains(got.Hint, dir) {
			t.Errorf("Hint = %q, want it to mention trusted dir %q", got.Hint, dir)
		}
	}
}

func TestTrySkipDisabledOptionAlwaysRunsFull(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-1|x|-|-|\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	cur, err := readFingerprint(nil, pkgList, scanProfile(false))
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	findings, ok := trySkip(Options{SkipIfUnchanged: false}, nil, pkgList, fingerprintPath, reportPath)
	if ok {
		t.Error("trySkip should never skip when SkipIfUnchanged is off")
	}
	if findings != nil {
		t.Errorf("findings = %v, want nil when not skipping", findings)
	}
}

func TestTrySkipReusesReportWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-7408|Harden SSH config|-|Change PermitRootLogin|\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	opts := Options{SkipIfUnchanged: true}
	cur, err := readFingerprint(nil, pkgList, scanProfile(opts.Quick))
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	findings, ok := trySkip(opts, nil, pkgList, fingerprintPath, reportPath)
	if !ok {
		t.Fatal("trySkip should skip and reuse the report when nothing changed")
	}
	if len(findings) != 1 || findings[0].TestID != "SSH-7408" {
		t.Errorf("findings = %+v, want the report's SSH-7408 finding", findings)
	}
}

func TestTrySkipFallsThroughWhenChanged(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-1|x|-|-|\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	opts := Options{SkipIfUnchanged: true}
	cur, err := readFingerprint(nil, pkgList, scanProfile(opts.Quick))
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}
	writeFileT(t, pkgList, "Package: foo\nPackage: bar\n")

	if _, ok := trySkip(opts, nil, pkgList, fingerprintPath, reportPath); ok {
		t.Error("trySkip should not skip once the package list changed")
	}
}

func TestPersistFingerprintDisabledOptionIsNoop(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	persistFingerprint(Options{SkipIfUnchanged: false}, nil, pkgList, fingerprintPath)

	got, err := loadFingerprint(fingerprintPath)
	if err != nil {
		t.Fatalf("loadFingerprint: %v", err)
	}
	if got != "" {
		t.Errorf("loadFingerprint = %q, want empty: persistFingerprint should be a no-op when SkipIfUnchanged is off", got)
	}
}

func TestTrySkipReturnsFalseWhenCachedReportFailsIntegrityCheck(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-7408|Harden SSH config|-|Change PermitRootLogin|\n")
	if err := os.Chmod(reportPath, 0o646); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	opts := Options{SkipIfUnchanged: true}
	cur, err := readFingerprint(nil, pkgList, scanProfile(opts.Quick))
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	findings, ok := trySkip(opts, nil, pkgList, fingerprintPath, reportPath)
	if ok {
		t.Error("trySkip should not skip when the cached report fails its integrity check")
	}
	if findings != nil {
		t.Errorf("findings = %v, want nil when the cached report fails integrity check", findings)
	}
}

func TestRunLynisAuditSucceeds(t *testing.T) {
	bin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no 'true' binary on PATH")
	}
	if runErr := runLynisAudit(t.Context(), bin, Options{}); runErr != nil {
		t.Fatalf("runLynisAudit: %v", runErr)
	}
}

func TestRunLynisAuditErrorsWhenItCannotStart(t *testing.T) {
	// An already-canceled context makes cmd.Start fail outright (not an
	// *exec.ExitError), which must surface as a plain error rather than
	// being tolerated the way a non-zero exit (suggestions/warnings) is.
	// Using a nonexistent binary path isn't reliable here: when "nice" is
	// present (as on virtually every Linux/macOS box), priorityWrap wraps
	// the missing binary behind nice, which itself starts fine and merely
	// exits non-zero — indistinguishable from a tolerated lynis exit.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := runLynisAudit(ctx, "/bin/true", Options{}); err == nil {
		t.Fatal("expected an error when the command can't be started")
	}
}

func TestRunLynisAuditToleratesNonZeroExit(t *testing.T) {
	// lynis exits non-zero whenever it has suggestions/warnings; that must
	// be tolerated, same as a genuine lynis run.
	bin, err := exec.LookPath("false")
	if err != nil {
		t.Skip("no 'false' binary on PATH")
	}
	if runErr := runLynisAudit(t.Context(), bin, Options{}); runErr != nil {
		t.Fatalf("expected a non-zero exit to be tolerated, got error: %v", runErr)
	}
}

func TestPersistFingerprintSavesCurrentState(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")
	opts := Options{SkipIfUnchanged: true, Quick: true}

	persistFingerprint(opts, nil, pkgList, fingerprintPath)

	want, err := readFingerprint(nil, pkgList, scanProfile(opts.Quick))
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	got, err := loadFingerprint(fingerprintPath)
	if err != nil {
		t.Fatalf("loadFingerprint: %v", err)
	}
	if got != want {
		t.Errorf("loadFingerprint = %q, want %q", got, want)
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
