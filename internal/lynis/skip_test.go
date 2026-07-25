package lynis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintHashStableForSameInputs(t *testing.T) {
	a := []fingerprintInput{{path: "/etc/ssh/sshd_config", content: []byte("PermitRootLogin no\n")}}
	b := []fingerprintInput{{path: "/etc/ssh/sshd_config", content: []byte("PermitRootLogin no\n")}}
	if fingerprintHash(a) != fingerprintHash(b) {
		t.Error("hash of separately-built but equal inputs should be equal")
	}
}

func TestFingerprintHashChangesWithContent(t *testing.T) {
	a := []fingerprintInput{{path: "/etc/ssh/sshd_config", content: []byte("PermitRootLogin no\n")}}
	b := []fingerprintInput{{path: "/etc/ssh/sshd_config", content: []byte("PermitRootLogin yes\n")}}
	if fingerprintHash(a) == fingerprintHash(b) {
		t.Error("hash should change when file content changes")
	}
}

func TestFingerprintHashAbsentDiffersFromEmpty(t *testing.T) {
	absent := []fingerprintInput{{path: "/etc/fail2ban/jail.local", absent: true}}
	empty := []fingerprintInput{{path: "/etc/fail2ban/jail.local", content: []byte{}}}
	if fingerprintHash(absent) == fingerprintHash(empty) {
		t.Error("an absent file should hash differently than a present-but-empty one")
	}
}

func TestReadFingerprintToleratesMissingConfigFile(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.conf")
	if err := os.WriteFile(present, []byte("hello"), 0o644); err != nil {
		t.Fatalf("seeding config file: %v", err)
	}
	missing := filepath.Join(dir, "does-not-exist.conf")
	pkgList := filepath.Join(dir, "dpkg-status")
	if err := os.WriteFile(pkgList, []byte("Package: foo\n"), 0o644); err != nil {
		t.Fatalf("seeding pkg list: %v", err)
	}

	fp, err := readFingerprint([]string{present, missing}, pkgList, "full")
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if fp == "" {
		t.Fatal("expected a non-empty fingerprint")
	}
}

func TestReadFingerprintChangesWhenPkgListChanges(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	before, err := readFingerprint(nil, pkgList, "full")
	if err != nil {
		t.Fatalf("readFingerprint (before): %v", err)
	}

	writeFileT(t, pkgList, "Package: foo\nPackage: bar\n")
	after, err := readFingerprint(nil, pkgList, "full")
	if err != nil {
		t.Fatalf("readFingerprint (after): %v", err)
	}
	if before == after {
		t.Error("fingerprint should change when the package list file's size/mtime changes")
	}
}

func TestSaveAndLoadFingerprintRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "lynis-fingerprint.txt")
	if err := saveFingerprint(path, "abc123"); err != nil {
		t.Fatalf("saveFingerprint: %v", err)
	}
	got, err := loadFingerprint(path)
	if err != nil {
		t.Fatalf("loadFingerprint: %v", err)
	}
	if got != "abc123" {
		t.Errorf("loadFingerprint = %q, want %q", got, "abc123")
	}
}

func TestLoadFingerprintMissingCacheIsNotAnError(t *testing.T) {
	got, err := loadFingerprint(filepath.Join(t.TempDir(), "does-not-exist.txt"))
	if err != nil {
		t.Fatalf("loadFingerprint: %v", err)
	}
	if got != "" {
		t.Errorf("loadFingerprint = %q, want empty string for a missing cache", got)
	}
}

func TestShouldSkipNoCacheYet(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-1|x|-|-|\n")

	skip, err := shouldSkip(nil, pkgList, filepath.Join(dir, "fingerprint.txt"), reportPath, "full")
	if err != nil {
		t.Fatalf("shouldSkip: %v", err)
	}
	if skip {
		t.Error("should not skip when there is no cached fingerprint yet")
	}
}

func TestShouldSkipMatchingCacheAndReportPresent(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-1|x|-|-|\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	cur, err := readFingerprint(nil, pkgList, "full")
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	skip, err := shouldSkip(nil, pkgList, fingerprintPath, reportPath, "full")
	if err != nil {
		t.Fatalf("shouldSkip: %v", err)
	}
	if !skip {
		t.Error("should skip when the cached fingerprint matches and a report exists")
	}
}

func TestShouldSkipMatchingCacheButReportMissing(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	cur, err := readFingerprint(nil, pkgList, "full")
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	skip, err := shouldSkip(nil, pkgList, fingerprintPath, filepath.Join(dir, "no-report.dat"), "full")
	if err != nil {
		t.Fatalf("shouldSkip: %v", err)
	}
	if skip {
		t.Error("should not skip when the last report is no longer on disk")
	}
}

func TestShouldSkipConfigChangedSinceCache(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-1|x|-|-|\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	cur, err := readFingerprint(nil, pkgList, "full")
	if err != nil {
		t.Fatalf("readFingerprint: %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, cur); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	writeFileT(t, pkgList, "Package: foo\nPackage: bar\n")

	skip, err := shouldSkip(nil, pkgList, fingerprintPath, reportPath, "full")
	if err != nil {
		t.Fatalf("shouldSkip: %v", err)
	}
	if skip {
		t.Error("should not skip when the package list changed since the cached fingerprint")
	}
}

func TestFingerprintHashDiffersByProfile(t *testing.T) {
	pkgList := filepath.Join(t.TempDir(), "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")

	quick, err := readFingerprint(nil, pkgList, scanProfile(true))
	if err != nil {
		t.Fatalf("readFingerprint (quick): %v", err)
	}
	full, err := readFingerprint(nil, pkgList, scanProfile(false))
	if err != nil {
		t.Fatalf("readFingerprint (full): %v", err)
	}
	if quick == full {
		t.Error("fingerprint should differ between quick and full profiles even with identical file state")
	}
}

// TestShouldSkipNeverServesOtherProfilesCache reproduces the request-changes
// scenario: a cached fingerprint saved after a --quick scan must not be
// treated as "unchanged" for a later full-scan request, and vice versa,
// even though the underlying config/pkg-list state never changed. Serving
// either cache across profiles would silently hand back a lighter (or
// stale-relative-to-full) report than the caller asked for.
func TestShouldSkipNeverServesOtherProfilesCache(t *testing.T) {
	dir := t.TempDir()
	pkgList := filepath.Join(dir, "dpkg-status")
	writeFileT(t, pkgList, "Package: foo\n")
	reportPath := filepath.Join(dir, "report.dat")
	writeFileT(t, reportPath, "suggestion[]=SSH-1|x|-|-|\n")
	fingerprintPath := filepath.Join(dir, "fingerprint.txt")

	quickFP, err := readFingerprint(nil, pkgList, scanProfile(true))
	if err != nil {
		t.Fatalf("readFingerprint (quick): %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, quickFP); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	skip, err := shouldSkip(nil, pkgList, fingerprintPath, reportPath, scanProfile(false))
	if err != nil {
		t.Fatalf("shouldSkip (full request against quick cache): %v", err)
	}
	if skip {
		t.Error("a full-scan request must not skip using a cache saved by a --quick scan")
	}

	fullFP, err := readFingerprint(nil, pkgList, scanProfile(false))
	if err != nil {
		t.Fatalf("readFingerprint (full): %v", err)
	}
	if saveErr := saveFingerprint(fingerprintPath, fullFP); saveErr != nil {
		t.Fatalf("saveFingerprint: %v", saveErr)
	}

	skip, err = shouldSkip(nil, pkgList, fingerprintPath, reportPath, scanProfile(true))
	if err != nil {
		t.Fatalf("shouldSkip (quick request against full cache): %v", err)
	}
	if skip {
		t.Error("a --quick request must not skip using a cache saved by a full scan")
	}
}

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
