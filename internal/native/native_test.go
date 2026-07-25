package native

import "testing"

func TestDpkgStatusInstalled(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{"installed", "install ok installed", true},
		{"installed with trailing newline", "install ok installed\n", true},
		{"removed but not purged, conffiles remain", "deinstall ok config-files", false},
		{"half-installed", "install ok half-installed", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dpkgStatusInstalled(tc.status); got != tc.want {
				t.Errorf("dpkgStatusInstalled(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
