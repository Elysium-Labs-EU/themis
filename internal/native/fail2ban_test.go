package native

import (
	"context"
	"errors"
	"testing"
)

func TestFail2banDecision(t *testing.T) {
	cases := []struct {
		name            string
		active          bool
		jailEnabled     bool
		banactionScoped bool
		wantFinding     bool
	}{
		{"inactive", false, false, false, true},
		{"active but no jail", true, false, false, true},
		{"active with jail but unscoped banaction", true, true, false, true},
		{"active with jail and scoped banaction", true, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := fail2banDecision(tc.active, tc.jailEnabled, tc.banactionScoped)
			if (f != nil) != tc.wantFinding {
				t.Fatalf("fail2banDecision(%v, %v, %v) = %v, want finding=%v", tc.active, tc.jailEnabled, tc.banactionScoped, f, tc.wantFinding)
			}
			if f != nil && f.TestID != "THEMIS-FAIL2BAN" {
				t.Errorf("TestID = %q, want THEMIS-FAIL2BAN", f.TestID)
			}
			if f != nil && f.Source != "themis" {
				t.Errorf("Source = %q, want themis", f.Source)
			}
		})
	}
}

func TestFail2banFinding(t *testing.T) {
	// fail2banJailLocalPath is a fixed absolute host path that doesn't
	// exist on a dev/CI box, so the active branch deterministically
	// falls through to "no enabled sshd jail".
	cases := []struct {
		name            string
		systemctl       stubResult
		wantDescription string
	}{
		{"fail2ban inactive", stubResult{err: errors.New("inactive")}, "fail2ban is not installed or not active"},
		{"fail2ban active but unconfigured host", stubResult{err: nil}, "fail2ban is active but has no enabled sshd jail"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := stubRunner(t, map[string]stubResult{"systemctl": tc.systemctl})

			f, err := fail2banFinding(context.Background(), runner)
			if err != nil {
				t.Fatalf("fail2banFinding: %v", err)
			}
			if f == nil {
				t.Fatal("expected a finding")
			}
			if f.TestID != "THEMIS-FAIL2BAN" {
				t.Errorf("TestID = %q, want THEMIS-FAIL2BAN", f.TestID)
			}
			if f.Description != tc.wantDescription {
				t.Errorf("Description = %q, want %q", f.Description, tc.wantDescription)
			}
		})
	}
}
