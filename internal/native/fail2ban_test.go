package native

import "testing"

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
