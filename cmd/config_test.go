package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/config"
)

// runConfig runs `themis config <args...>` against a fresh command tree,
// capturing stdout. Returns the trimmed output and any run error.
func runConfig(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newConfigCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return strings.TrimSpace(out.String()), err
}

func TestConfigPath_PrintsResolvedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvVar, path)

	got, err := runConfig(t, "path")
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if got != path {
		t.Errorf("config path = %q, want %q", got, path)
	}
}

func TestConfigGet_MissingFileReturnsDefault(t *testing.T) {
	t.Setenv(config.EnvVar, filepath.Join(t.TempDir(), "config.yaml"))

	got, err := runConfig(t, "get", "sources.lynis.enabled")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if got != "true" {
		t.Errorf("config get default = %q, want true", got)
	}
}

func TestConfigGet_UnknownKeyErrors(t *testing.T) {
	t.Setenv(config.EnvVar, filepath.Join(t.TempDir(), "config.yaml"))

	if _, err := runConfig(t, "get", "sources.bogus.enabled"); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}

// TestConfigSetThenGet is the brief's core acceptance path at the command
// layer: set a value, read it back, and confirm the file was written from
// defaults with only that key changed.
func TestConfigSetThenGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvVar, path)

	if _, err := runConfig(t, "set", "sources.lynis.enabled", "false"); err != nil {
		t.Fatalf("config set: %v", err)
	}

	got, err := runConfig(t, "get", "sources.lynis.enabled")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if got != "false" {
		t.Errorf("config get after set = %q, want false", got)
	}

	// The written file loads back with lynis disabled and every other
	// source at its default — proving set created a complete config from
	// defaults rather than a sparse one.
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sources.Lynis.Enabled {
		t.Error("expected persisted sources.lynis.enabled false")
	}
	if !cfg.Sources.Native.Enabled {
		t.Error("expected sources.native.enabled to keep its default true")
	}
}

func TestConfigSet_UnknownKeyExitsNonZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvVar, path)

	if _, err := runConfig(t, "set", "sources.bogus.enabled", "true"); err == nil {
		t.Fatal("expected a non-nil error (non-zero exit) for an unknown key")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("config file should not be written when the key is rejected")
	}
}

func TestConfigSet_InvalidBooleanErrors(t *testing.T) {
	t.Setenv(config.EnvVar, filepath.Join(t.TempDir(), "config.yaml"))

	if _, err := runConfig(t, "set", "sources.lynis.enabled", "notabool"); err == nil {
		t.Fatal("expected an error for a non-boolean value")
	}
}

func TestConfigSet_StringKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvVar, path)

	if _, err := runConfig(t, "set", "sources.openscap.content", "/opt/ssg.xml"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	got, err := runConfig(t, "get", "sources.openscap.content")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if got != "/opt/ssg.xml" {
		t.Errorf("config get = %q, want /opt/ssg.xml", got)
	}
}
