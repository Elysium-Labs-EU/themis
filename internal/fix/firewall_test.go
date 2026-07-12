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
