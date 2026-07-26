package cmd

import (
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
	"github.com/Elysium-Labs-EU/themis/internal/config"
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

// TestEnabledSourcesMatchHistoricalOrder pins the enabled set built from
// config.Defaults() (no config file, no flags) to what cmd/check.go's old
// inline sources() built: lynis, themis-native, osquery — with openscap
// appended only when a SCAP content path is configured. This is what
// keeps `themis check` output identical after moving to the registry.
func TestEnabledSourcesMatchHistoricalOrder(t *testing.T) {
	got := enabledNames(t, checkSourceConfig(config.Defaults(), checkFlags{}))
	want := []string{"lynis", "themis", "osquery"}
	if !equalStrings(got, want) {
		t.Fatalf("default enabled sources = %v, want %v", got, want)
	}

	withContent := config.Defaults()
	withContent.Sources.OpenSCAP.Enabled = true
	withContent.Sources.OpenSCAP.Content = "/content.xml"
	got = enabledNames(t, checkSourceConfig(withContent, checkFlags{}))
	want = []string{"lynis", "themis", "osquery", "openscap"}
	if !equalStrings(got, want) {
		t.Fatalf("enabled sources with SCAP content = %v, want %v", got, want)
	}
}

// TestConfigLynisDisabledDropsLynisEvenWithNoFlags is the issue #82
// acceptance test: sources.lynis.enabled: false in the operator config,
// with no CLI flags, drops lynis from the enabled set while the other
// default sources keep running.
func TestConfigLynisDisabledDropsLynisEvenWithNoFlags(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sources.Lynis.Enabled = false

	got := enabledNames(t, checkSourceConfig(cfg, checkFlags{}))
	want := []string{"themis", "osquery"}
	if !equalStrings(got, want) {
		t.Fatalf("enabled sources with lynis disabled = %v, want %v", got, want)
	}
}

// TestQuickFlagOverridesConfigFileQuick proves flags win over the config
// file: sources.lynis.quick: false in config, --quick passed, expects
// quick=true in the resolved SourceConfig.
func TestQuickFlagOverridesConfigFileQuick(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sources.Lynis.Quick = false

	got := checkSourceConfig(cfg, checkFlags{Quick: true, QuickSet: true})
	if !got.LynisQuick {
		t.Fatal("expected --quick to override sources.lynis.quick: false from the config file")
	}
}

// TestConfigFileQuickAppliesWithNoFlag proves the config file's
// sources.lynis.quick takes effect when the CLI flag isn't passed at
// all.
func TestConfigFileQuickAppliesWithNoFlag(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sources.Lynis.Quick = true

	got := checkSourceConfig(cfg, checkFlags{})
	if !got.LynisQuick {
		t.Fatal("expected sources.lynis.quick: true from the config file to apply with no --quick flag")
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
