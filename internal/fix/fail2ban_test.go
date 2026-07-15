package fix

import (
	"strings"
	"testing"
)

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
			if got := sshdJailEnabled(tc.content); got != tc.want {
				t.Errorf("sshdJailEnabled(%q) = %v, want %v", tc.content, got, tc.want)
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
			if got := sshdBanactionScoped(tc.content); got != tc.want {
				t.Errorf("sshdBanactionScoped(%q) = %v, want %v", tc.content, got, tc.want)
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
	if !sshdJailEnabled(got) {
		t.Errorf("expected sshd jail enabled after ensureSSHDJail, got %q", got)
	}
	if !sshdBanactionScoped(got) {
		t.Errorf("expected banaction pinned to multiport after ensureSSHDJail, got %q", got)
	}

	already := "[sshd]\nenabled = true\nbackend = systemd\nbanaction = iptables-multiport\n"
	if got = ensureSSHDJail(already); got != already {
		t.Errorf("expected no change when already fully configured, got %q", got)
	}

	// enabled but missing banaction: must patch in place, not duplicate the section.
	partial := "[sshd]\nenabled = true\n"
	got = ensureSSHDJail(partial)
	if !sshdBanactionScoped(got) {
		t.Errorf("expected banaction to be added to existing [sshd] section, got %q", got)
	}
	if strings.Count(got, "[sshd]") != 1 {
		t.Errorf("expected exactly one [sshd] section, got %q", got)
	}

	// wrong banaction value: must be corrected in place, not appended as a duplicate line.
	wrong := "[sshd]\nenabled = true\nbanaction = iptables-allports\nport = ssh\n"
	got = ensureSSHDJail(wrong)
	if !sshdBanactionScoped(got) {
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
	if !sshdBanactionScoped(got) {
		t.Errorf("expected banaction added to [sshd], got %q", got)
	}
	if sectionHasKeyValue(got, "apache", "banaction", banactionMultiport) {
		t.Errorf("banaction leaked into the wrong section, got %q", got)
	}
}
