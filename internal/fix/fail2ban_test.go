package fix

import "testing"

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

func TestEnsureSSHDJail(t *testing.T) {
	got := ensureSSHDJail("")
	if !sshdJailEnabled(got) {
		t.Errorf("expected sshd jail enabled after ensureSSHDJail, got %q", got)
	}

	already := "[sshd]\nenabled = true\n"
	if got := ensureSSHDJail(already); got != already {
		t.Errorf("expected no change when already enabled, got %q", got)
	}
}
