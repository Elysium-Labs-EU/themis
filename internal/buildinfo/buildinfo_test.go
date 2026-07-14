package buildinfo

import (
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	origVersion, origCommit, origDate := Version, GitCommit, BuildDate
	t.Cleanup(func() { Version, GitCommit, BuildDate = origVersion, origCommit, origDate })

	Version = "v1.2.3"
	GitCommit = "abc123"
	BuildDate = "2026-07-14T00:00:00Z"

	got := Get()
	want := "v1.2.3 (commit: abc123, built: 2026-07-14T00:00:00Z)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGetVersionOnly(t *testing.T) {
	origVersion := Version
	t.Cleanup(func() { Version = origVersion })

	Version = "v9.9.9"
	if got := GetVersionOnly(); got != "v9.9.9" {
		t.Fatalf("got %q, want %q", got, "v9.9.9")
	}
}

func TestGetIncludesAllFields(t *testing.T) {
	got := Get()
	for _, want := range []string{Version, GitCommit, BuildDate} {
		if !strings.Contains(got, want) {
			t.Fatalf("Get() = %q, expected to contain %q", got, want)
		}
	}
}
