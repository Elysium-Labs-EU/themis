package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != Defaults() {
		t.Fatalf("Load(missing) = %+v, want Defaults() = %+v", got, Defaults())
	}
}

func TestLoadEmptyFileReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != Defaults() {
		t.Fatalf("Load(empty) = %+v, want Defaults() = %+v", got, Defaults())
	}
}

func TestLoadPartialFileKeepsDefaultsForOmittedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yamlBody := "sources:\n  lynis:\n    enabled: false\n"
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Sources.Lynis.Enabled {
		t.Error("expected sources.lynis.enabled: false to be honored")
	}
	if !got.Sources.Native.Enabled {
		t.Error("expected sources.native.enabled to keep its default (true) when omitted from the file")
	}
	if !got.Sources.Osquery.Enabled {
		t.Error("expected sources.osquery.enabled to keep its default (true) when omitted from the file")
	}
	if got.Sources.OpenSCAP.Enabled {
		t.Error("expected sources.openscap.enabled to keep its default (false) when omitted from the file")
	}
}

func TestLoadFullFileOverridesEveryField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yamlBody := `sources:
  lynis:    { enabled: true,  quick: true, skip_unchanged: true }
  native:   { enabled: false }
  osquery:  { enabled: false }
  openscap: { enabled: true, content: "/opt/ssg-content.xml", profile: "xccdf_org.ssgproject.content_profile_cis" }
`
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		Sources: SourcesConfig{
			Lynis:    LynisConfig{Enabled: true, Quick: true, SkipUnchanged: true},
			Native:   NativeConfig{Enabled: false},
			Osquery:  OsqueryConfig{Enabled: false},
			OpenSCAP: OpenSCAPConfig{Enabled: true, Content: "/opt/ssg-content.xml", Profile: "xccdf_org.ssgproject.content_profile_cis"},
		},
	}
	if got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadRejectsMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sources: [this is not a mapping"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected an error for malformed YAML")
	}
}

func TestPathHonorsEnvVarOverride(t *testing.T) {
	t.Setenv(EnvVar, "/custom/config.yaml")
	if got := Path(); got != "/custom/config.yaml" {
		t.Fatalf("Path() = %q, want /custom/config.yaml", got)
	}
}

func TestPathFallsBackToHomeDirWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; Path() takes the root branch instead")
	}
	t.Setenv(EnvVar, "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	want := filepath.Join(home, ".themis", "config.yaml")
	if got := Path(); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}
