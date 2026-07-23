package fix

import "testing"

func TestParseConnectionCIDR(t *testing.T) {
	cases := []struct {
		name          string
		sshConnection string
		wantCIDR      string
		wantOK        bool
	}{
		{"unset", "", "", false},
		{"ipv4", "203.0.113.5 51234 198.51.100.9 22", "203.0.113.5/32", true},
		{"ipv6", "2001:db8::5 51234 2001:db8::9 22", "2001:db8::5/128", true},
		{"garbage", "not-an-ip 1 2 3", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCIDR, gotOK := parseConnectionCIDR(tc.sshConnection)
			if gotCIDR != tc.wantCIDR || gotOK != tc.wantOK {
				t.Errorf("parseConnectionCIDR(%q) = (%q, %v), want (%q, %v)", tc.sshConnection, gotCIDR, gotOK, tc.wantCIDR, tc.wantOK)
			}
		})
	}
}

func TestCurrentConnectionCIDR(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "203.0.113.5 51234 198.51.100.9 22")
	cidr, ok := CurrentConnectionCIDR()
	if !ok || cidr != "203.0.113.5/32" {
		t.Errorf("CurrentConnectionCIDR() = (%q, %v), want (\"203.0.113.5/32\", true)", cidr, ok)
	}
}
