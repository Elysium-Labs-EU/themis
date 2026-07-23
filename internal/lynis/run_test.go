package lynis

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
