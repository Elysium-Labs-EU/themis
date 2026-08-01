package fix

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fail2banSvcFake models systemctl's active/enabled state machine (unlike
// fakeRunner in apply_revert_integration_test.go, which only tracks
// active), so Revert's is-active/is-enabled-driven branch can be exercised
// precisely.
type fail2banSvcFake struct {
	active  bool
	enabled bool
}

func (s *fail2banSvcFake) run(name string, args ...string) error {
	joined := name + " " + strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "is-active"):
		if !s.active {
			return errors.New("inactive")
		}
	case strings.Contains(joined, "is-enabled"):
		if !s.enabled {
			return errors.New("disabled")
		}
	case joined == "systemctl enable --now fail2ban":
		s.enabled = true
		s.active = true
	case joined == "systemctl disable --now fail2ban":
		s.enabled = false
		s.active = false
	case joined == "systemctl disable fail2ban":
		s.enabled = false
	case joined == "systemctl stop fail2ban":
		s.active = false
	case joined == "systemctl restart fail2ban":
		s.active = true
	}
	return nil
}

// TestFail2banRevertRestoresPriorServiceState is the regression test for
// issue #14: on a box where fail2ban was already installed but not
// active/enabled before apply (e.g. it ships in the base image but Debian
// 12+ needs backend=systemd to actually start), Revert used to
// unconditionally run "systemctl restart fail2ban", leaving the service
// permanently active+enabled regardless of its state before apply.
func TestFail2banRevertRestoresPriorServiceState(t *testing.T) {
	cases := []struct {
		name          string
		activeBefore  bool
		enabledBefore bool
	}{
		{"inactive and disabled (issue #14 repro)", false, false},
		{"active and enabled", true, true},
		{"active but disabled", true, false},
		{"inactive but enabled", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFail2banRevertRestoresState(t, tc.activeBefore, tc.enabledBefore)
		})
	}
}

// assertFail2banRevertRestoresState applies then reverts a fail2ban fix
// against a fake service starting in (activeBefore, enabledBefore) state,
// asserting Apply leaves it active+enabled and Revert restores the prior
// state exactly.
func assertFail2banRevertRestoresState(t *testing.T, activeBefore, enabledBefore bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jail.local")
	svc := &fail2banSvcFake{active: activeBefore, enabled: enabledBefore}
	f := fail2banFixWith(path, svc.run, func(string) bool { return true })

	data, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !svc.active || !svc.enabled {
		t.Fatalf("expected Apply to leave fail2ban active+enabled, got active=%v enabled=%v", svc.active, svc.enabled)
	}

	if err := f.Revert(data); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if svc.active != activeBefore {
		t.Errorf("active after revert = %v, want %v (pre-apply state)", svc.active, activeBefore)
	}
	if svc.enabled != enabledBefore {
		t.Errorf("enabled after revert = %v, want %v (pre-apply state)", svc.enabled, enabledBefore)
	}
}

// TestFail2banApplyLateFailureReturnsRevertData is the regression test for
// issue #126: a "systemctl enable --now fail2ban" failure after jail.local
// is already rewritten must not discard the already-known revert data, or a
// later `themis rollback` could never restore the prior config.
func TestFail2banApplyLateFailureReturnsRevertData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jail.local")
	prior := "[sshd]\nenabled = false\n"
	if err := os.WriteFile(path, []byte(prior), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := &fakeRunner{failOn: "enable --now"}
	f := fail2banFixWith(path, r.run, func(string) bool { return true })

	data, err := f.Apply()
	if err == nil {
		t.Fatal("expected Apply to fail when enable fails")
	}
	if data == nil {
		t.Fatal("expected non-nil revertData on a late failure after jail.local was already written")
	}

	written, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading written jail.local: %v", readErr)
	}
	if !SSHDJailEnabled(string(written)) {
		t.Fatalf("expected jail.local patched despite enable failure, got %q", written)
	}

	if revertErr := f.Revert(data); revertErr != nil {
		t.Fatalf("Revert: %v", revertErr)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading reverted jail.local: %v", readErr)
	}
	if string(got) != prior {
		t.Fatalf("expected Revert to restore pre-apply content %q, got %q", prior, got)
	}
}

func TestSSHDJailEnabled(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"enabled", "[sshd]\nenabled = true\nport = ssh\n", true},
		{"disabled", "[sshd]\nenabled = false\n", false},
		{"other section only", "[apache]\nenabled = true\n", false},
		{"enabled after other section", "[apache]\nenabled = true\n[sshd]\nenabled = true\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SSHDJailEnabled(tc.content); got != tc.want {
				t.Errorf("SSHDJailEnabled(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestSSHDBanactionScoped(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"multiport", "[sshd]\nenabled = true\nbanaction = iptables-multiport\n", true},
		{"allports", "[sshd]\nenabled = true\nbanaction = iptables-allports\n", false},
		{"missing", "[sshd]\nenabled = true\n", false},
		{"other section only", "[apache]\nbanaction = iptables-multiport\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SSHDBanactionScoped(tc.content); got != tc.want {
				t.Errorf("SSHDBanactionScoped(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestFail2banWarnMessage(t *testing.T) {
	if msg, detected := fail2banWarnMessage(false, false); detected || msg != "" {
		t.Errorf("fail2banWarnMessage(false, false) = (%q, %v), want (\"\", false)", msg, detected)
	}

	msg, detected := fail2banWarnMessage(true, false)
	if !detected {
		t.Fatalf("fail2banWarnMessage(true, false) detected = false, want true")
	}
	if !strings.Contains(msg, "WireGuard") || strings.Contains(msg, "CrowdSec") {
		t.Errorf("fail2banWarnMessage(true, false) = %q, want mention of WireGuard only", msg)
	}

	msg, detected = fail2banWarnMessage(false, true)
	if !detected {
		t.Fatalf("fail2banWarnMessage(false, true) detected = false, want true")
	}
	if !strings.Contains(msg, "CrowdSec") || strings.Contains(msg, "WireGuard") {
		t.Errorf("fail2banWarnMessage(false, true) = %q, want mention of CrowdSec only", msg)
	}

	msg, detected = fail2banWarnMessage(true, true)
	if !detected {
		t.Fatalf("fail2banWarnMessage(true, true) detected = false, want true")
	}
	if !strings.Contains(msg, "WireGuard") || !strings.Contains(msg, "CrowdSec") {
		t.Errorf("fail2banWarnMessage(true, true) = %q, want mention of both", msg)
	}
}

func TestEnsureSSHDJail(t *testing.T) {
	got := ensureSSHDJail("")
	if !SSHDJailEnabled(got) {
		t.Errorf("expected sshd jail enabled after ensureSSHDJail, got %q", got)
	}
	if !SSHDBanactionScoped(got) {
		t.Errorf("expected banaction pinned to multiport after ensureSSHDJail, got %q", got)
	}

	already := "[sshd]\nenabled = true\nbackend = systemd\nbanaction = iptables-multiport\n"
	if got = ensureSSHDJail(already); got != already {
		t.Errorf("expected no change when already fully configured, got %q", got)
	}

	// enabled but missing banaction: must patch in place, not duplicate the section.
	partial := "[sshd]\nenabled = true\n"
	got = ensureSSHDJail(partial)
	if !SSHDBanactionScoped(got) {
		t.Errorf("expected banaction to be added to existing [sshd] section, got %q", got)
	}
	if strings.Count(got, "[sshd]") != 1 {
		t.Errorf("expected exactly one [sshd] section, got %q", got)
	}

	// wrong banaction value: must be corrected in place, not appended as a duplicate line.
	wrong := "[sshd]\nenabled = true\nbanaction = iptables-allports\nport = ssh\n"
	got = ensureSSHDJail(wrong)
	if !SSHDBanactionScoped(got) {
		t.Errorf("expected banaction to be corrected to multiport, got %q", got)
	}
	if strings.Count(got, "banaction") != 1 {
		t.Errorf("expected exactly one banaction line, got %q", got)
	}
	if !strings.Contains(got, "port = ssh") {
		t.Errorf("expected unrelated keys in the section to be preserved, got %q", got)
	}

	// [sshd] followed by another section: patch must insert before it, not inside it.
	beforeNext := "[sshd]\nenabled = true\n[apache]\nenabled = true\n"
	got = ensureSSHDJail(beforeNext)
	if !SSHDBanactionScoped(got) {
		t.Errorf("expected banaction added to [sshd], got %q", got)
	}
	if sectionHasKeyValue(got, "apache", "banaction", banactionMultiport) {
		t.Errorf("banaction leaked into the wrong section, got %q", got)
	}
}

func TestEnsureIgnoreIP(t *testing.T) {
	// No CIDR requested: no-op, even against an empty config.
	if got := ensureIgnoreIP("[sshd]\nenabled = true\n", ""); got != "[sshd]\nenabled = true\n" {
		t.Errorf("expected no-op with empty cidr, got %q", got)
	}

	// No [DEFAULT] section yet: create one, seeding the loopback baseline
	// that a bare "ignoreip = <cidr>" would otherwise silently replace.
	got := ensureIgnoreIP("[sshd]\nenabled = true\n", "203.0.113.5/32")
	if !sectionHasKeyValue(got, "DEFAULT", "ignoreip", "127.0.0.1/8 ::1 203.0.113.5/32") {
		t.Errorf("expected seeded ignoreip line in new [DEFAULT] section, got %q", got)
	}

	// [DEFAULT] exists but has no ignoreip key: same seeding, patched in
	// place rather than duplicating the section.
	got = ensureIgnoreIP("[DEFAULT]\nbantime = 3600\n[sshd]\nenabled = true\n", "203.0.113.5/32")
	if !sectionHasKeyValue(got, "DEFAULT", "ignoreip", "127.0.0.1/8 ::1 203.0.113.5/32") {
		t.Errorf("expected seeded ignoreip line added to existing [DEFAULT], got %q", got)
	}
	if strings.Count(got, "[DEFAULT]") != 1 {
		t.Errorf("expected exactly one [DEFAULT] section, got %q", got)
	}

	// Existing ignoreip line: append, don't clobber what's already there.
	existing := "[DEFAULT]\nignoreip = 127.0.0.1/8 ::1 198.51.100.0/24\n"
	got = ensureIgnoreIP(existing, "203.0.113.5/32")
	if !sectionHasKeyValue(got, "DEFAULT", "ignoreip", "127.0.0.1/8 ::1 198.51.100.0/24 203.0.113.5/32") {
		t.Errorf("expected cidr appended to existing ignoreip line, got %q", got)
	}

	// Already exempted: no-op, no duplicate token.
	already := "[DEFAULT]\nignoreip = 127.0.0.1/8 ::1 203.0.113.5/32\n"
	if got = ensureIgnoreIP(already, "203.0.113.5/32"); got != already {
		t.Errorf("expected no change when cidr already exempted, got %q", got)
	}
}
