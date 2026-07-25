package lynis

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FingerprintPath is where the fingerprint from the last full lynis scan
// is cached, so a later SkipIfUnchanged run can tell whether anything
// lynis cares about has changed since.
const FingerprintPath = "/var/lib/themis/lynis-fingerprint.txt"

// dpkgStatusPath is dpkg's package database. Its mtime and size change on
// every package install/remove/upgrade, which is a cheap proxy for "the
// package list lynis inspects has changed" without hashing the whole
// (often multi-MB) file.
const dpkgStatusPath = "/var/lib/dpkg/status"

// fingerprintPaths are the config files whose content affects what a
// lynis audit would find. Paths mirror the config files internal/fix and
// internal/native already track; duplicated here as plain literals
// (rather than exported constants) so lynis doesn't gain a dependency on
// those packages just to reuse a string.
var fingerprintPaths = []string{
	"/etc/ssh/sshd_config",
	"/etc/fail2ban/jail.local",
	"/etc/apt/apt.conf.d/20auto-upgrades",
	"/etc/apt/apt.conf.d/50unattended-upgrades",
	"/etc/sysctl.d/60-themis-hardening.conf",
}

// fingerprintInput is one file's contribution to a fingerprint. absent is
// tracked separately from present-but-empty, since e.g. fail2ban not
// being installed and its jail.local existing but empty are different
// states worth re-scanning for.
type fingerprintInput struct {
	path    string
	content []byte
	absent  bool
}

// fingerprintHash hashes ins into a single digest. Pure — no I/O.
func fingerprintHash(ins []fingerprintInput) string {
	h := sha256.New()
	for _, in := range ins {
		_, _ = fmt.Fprintf(h, "%s\x00", in.path)
		if in.absent {
			_, _ = h.Write([]byte("absent"))
		} else {
			_, _ = h.Write(in.content)
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// scanProfile labels which lynis profile opts.Quick selects, so it can be
// folded into the fingerprint below — a quick scan and a full scan can
// disagree on the same host state, so a cached result from one must
// never be served for a request for the other. Pure — no I/O.
func scanProfile(quick bool) string {
	if quick {
		return "quick"
	}
	return "full"
}

// readFingerprint reads configPaths and stats pkgListPath, then hashes
// them (plus profile, see scanProfile) into a single fingerprint of
// everything lynis's scan output depends on. A missing config file is not
// an error (e.g. fail2ban isn't installed on every host) — it's folded
// into the fingerprint as absent instead.
func readFingerprint(configPaths []string, pkgListPath, profile string) (string, error) {
	ins := make([]fingerprintInput, 0, len(configPaths)+2)
	ins = append(ins, fingerprintInput{path: "lynis:profile", content: []byte(profile)})
	for _, p := range configPaths {
		b, err := os.ReadFile(p) //nolint:gosec // p comes from a fixed set of config-path constants, not user input
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				ins = append(ins, fingerprintInput{path: p, absent: true})
				continue
			}
			return "", fmt.Errorf("reading %s: %w", p, err)
		}
		ins = append(ins, fingerprintInput{path: p, content: b})
	}

	info, err := os.Stat(pkgListPath)
	switch {
	case err == nil:
		ins = append(ins, fingerprintInput{
			path:    pkgListPath,
			content: fmt.Appendf(nil, "%d:%d", info.Size(), info.ModTime().UnixNano()),
		})
	case errors.Is(err, os.ErrNotExist):
		ins = append(ins, fingerprintInput{path: pkgListPath, absent: true})
	default:
		return "", fmt.Errorf("statting %s: %w", pkgListPath, err)
	}

	return fingerprintHash(ins), nil
}

// loadFingerprint reads the fingerprint cached at path. A missing cache
// (first run) is not an error — it just means there's nothing to compare
// against yet, so the caller should not skip.
func loadFingerprint(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is the fixed fingerprint-cache constant, not user input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading lynis fingerprint cache %s: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// saveFingerprint persists fp to path, creating parent directories as
// needed.
func saveFingerprint(path, fp string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating fingerprint cache dir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(fp), 0o600); err != nil {
		return fmt.Errorf("writing lynis fingerprint cache %s: %w", path, err)
	}
	return nil
}

// shouldSkip reports whether a lynis re-scan can be skipped: the current
// fingerprint of configPaths/pkgListPath/profile matches what was
// persisted at fingerprintCachePath after the last scan with that same
// profile, and that scan's report is still on disk at reportPath to
// reuse. profile is folded into the fingerprint (see scanProfile) so a
// quick-scan request never reuses a full scan's cached report, or vice
// versa.
func shouldSkip(configPaths []string, pkgListPath, fingerprintCachePath, reportPath, profile string) (bool, error) {
	cur, err := readFingerprint(configPaths, pkgListPath, profile)
	if err != nil {
		return false, err
	}
	prev, err := loadFingerprint(fingerprintCachePath)
	if err != nil {
		return false, err
	}
	if prev == "" || prev != cur {
		return false, nil
	}
	if _, statErr := os.Stat(reportPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return false, nil
		}
		return false, statErr
	}
	return true, nil
}
