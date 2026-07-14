package fix

import "testing"

func TestLynisTestIDFallsBackToTestID(t *testing.T) {
	f := Fix{TestID: "FIRE-4590"}
	if got := f.LynisTestID(); got != "FIRE-4590" {
		t.Fatalf("got %q, want %q", got, "FIRE-4590")
	}
}

func TestLynisTestIDPrefersLynisIDWhenSet(t *testing.T) {
	f := Fix{TestID: "SSH-7408-PASSWDAUTH", LynisID: "SSH-7408"}
	if got := f.LynisTestID(); got != "SSH-7408" {
		t.Fatalf("got %q, want %q", got, "SSH-7408")
	}
}

func TestRegistryEntriesHaveConsistentTestIDs(t *testing.T) {
	for key, f := range Registry {
		if f.TestID != key {
			t.Errorf("Registry[%q] has TestID %q, want it to match its key", key, f.TestID)
		}
		if f.Check == nil || f.Apply == nil || f.Revert == nil {
			t.Errorf("Registry[%q] is missing a Check/Apply/Revert func", key)
		}
	}
}
