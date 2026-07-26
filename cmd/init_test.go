package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/config"
)

// TestRunInitYesWritesLoadableDefaults is the acceptance criterion: on a
// host with no config, `themis init --yes` writes a config the loader
// (what `themis check` uses) reads back as the built-in defaults.
func TestRunInitYesWritesLoadableDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "themis", "config.yaml")

	buf := &bytes.Buffer{}
	if err := runInit(strings.NewReader(""), buf, path, true); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load after init --yes: %v", err)
	}
	if got != config.Defaults() {
		t.Fatalf("loaded config = %+v, want Defaults() = %+v", got, config.Defaults())
	}
}

// TestRunInitWithoutYesRefusesToClobberWhenDeclined covers the re-run
// guard: an existing file plus a declined overwrite leaves it untouched.
func TestRunInitWithoutYesRefusesToClobberWhenDeclined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("sources:\n  lynis:\n    enabled: false\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := runInit(strings.NewReader("n\n"), buf, path, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("file was modified after a declined overwrite: %q", after)
	}
	if !strings.Contains(buf.String(), "Canceled.") {
		t.Errorf("output = %q, want a Canceled. message", buf.String())
	}
}

// TestRunInitPromptsBuildConfigFromAnswers checks the wizard threads its
// answers through to the written file: enabling openscap and disabling
// osquery via prompts must land in what the loader reads back.
func TestRunInitPromptsBuildConfigFromAnswers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	// Prompt order: lynis enabled? / quick? / skip_unchanged? / native? /
	// osquery? / openscap? / content / profile.
	answers := strings.Join([]string{
		"y",                    // lynis enabled
		"n",                    // lynis quick
		"n",                    // lynis skip_unchanged
		"y",                    // native enabled
		"n",                    // osquery enabled -> disabled
		"y",                    // openscap enabled
		"/opt/ssg-content.xml", // openscap content
		"",                     // openscap profile -> keep default (empty)
	}, "\n") + "\n"

	buf := &bytes.Buffer{}
	if err := runInit(strings.NewReader(answers), buf, path, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if got.Sources.Osquery.Enabled {
		t.Error("expected osquery disabled from prompt answer")
	}
	if !got.Sources.OpenSCAP.Enabled || got.Sources.OpenSCAP.Content != "/opt/ssg-content.xml" {
		t.Errorf("expected openscap enabled with content path, got %+v", got.Sources.OpenSCAP)
	}
}
