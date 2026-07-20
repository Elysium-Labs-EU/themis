package fix

import "testing"

func TestParseDefaultIncoming(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{"deny", "Status: active\nLogging: on (low)\nDefault: deny (incoming), allow (outgoing), disabled (routed)\n", "deny"},
		{"allow", "Default: allow (incoming), deny (outgoing)\n", "allow"},
		{"reject", "Default: reject (incoming), deny (outgoing)\n", "reject"},
		{"none found", "Status: inactive\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseDefaultIncoming(tc.output); got != tc.want {
				t.Errorf("parseDefaultIncoming(%q) = %q, want %q", tc.output, got, tc.want)
			}
		})
	}
}

func TestSSHAllowPorts(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   []string
	}{
		{"empty config defaults to 22", "", []string{"22"}},
		{"no Port directive defaults to 22", "PermitRootLogin no\n", []string{"22"}},
		{"single custom port", "Port 2222\n", []string{"2222"}},
		{"commented out Port defaults to 22", "#Port 2222\n", []string{"22"}},
		{"case-insensitive directive name", "port 2222\n", []string{"2222"}},
		{
			"multiple Port lines are cumulative, not last-wins",
			"Port 22\nPort 2222\n",
			[]string{"22", "2222"},
		},
		{
			"inline comment after value doesn't break parsing",
			"Port 22 # primary\n",
			[]string{"22"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sshAllowPorts(tc.config)
			if len(got) != len(tc.want) {
				t.Fatalf("sshAllowPorts(%q) = %v, want %v", tc.config, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("sshAllowPorts(%q) = %v, want %v", tc.config, got, tc.want)
				}
			}
		})
	}
}

func TestUfwAllowsPort(t *testing.T) {
	cases := []struct {
		name   string
		status string
		port   string
		want   bool
	}{
		{"no rules at all", "Status: active\nDefault: deny (incoming), allow (outgoing)\n", "22", false},
		{
			"explicit port allow",
			"Status: active\nDefault: deny (incoming), allow (outgoing)\n\nTo                         Action      From\n--                         ------      ----\n22/tcp                     ALLOW IN    Anywhere\n",
			"22", true,
		},
		{
			"OpenSSH app profile counts for port 22",
			"Status: active\nDefault: deny (incoming), allow (outgoing)\n\nTo                         Action      From\n--                         ------      ----\nOpenSSH                    ALLOW IN    Anywhere\n",
			"22", true,
		},
		{
			"OpenSSH profile does not count for a non-standard port",
			"OpenSSH                    ALLOW IN    Anywhere\n",
			"2222", false,
		},
		{
			"DENY rule for the port doesn't count as allowed",
			"22/tcp                     DENY IN     Anywhere\n",
			"22", false,
		},
		{
			"allow rule for a different port doesn't match",
			"2222/tcp                   ALLOW IN    Anywhere\n",
			"22", false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ufwAllowsPort(tc.status, tc.port); got != tc.want {
				t.Errorf("ufwAllowsPort(%q, %q) = %v, want %v", tc.status, tc.port, got, tc.want)
			}
		})
	}
}
