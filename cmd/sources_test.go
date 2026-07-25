package cmd

import (
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
)

func enabledNames(t *testing.T, cfg audit.SourceConfig) []string {
	t.Helper()
	srcs, err := audit.Enabled(cfg)
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	names := make([]string, len(srcs))
	for i, s := range srcs {
		names[i] = s.Name()
	}
	return names
}

// TestEnabledSourcesMatchHistoricalOrder pins the enabled set the real
// (blank-imported) source registrations produce to what cmd/check.go's
// old inline sources() built: lynis, themis-native, osquery — with
// openscap appended only when a SCAP content path is configured. This is
// what keeps `themis check` output identical after moving to the registry.
func TestEnabledSourcesMatchHistoricalOrder(t *testing.T) {
	got := enabledNames(t, checkSourceConfig(false, false, "", ""))
	want := []string{"lynis", "themis", "osquery"}
	if !equalStrings(got, want) {
		t.Fatalf("default enabled sources = %v, want %v", got, want)
	}

	got = enabledNames(t, checkSourceConfig(false, false, "/content.xml", ""))
	want = []string{"lynis", "themis", "osquery", "openscap"}
	if !equalStrings(got, want) {
		t.Fatalf("enabled sources with SCAP content = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
