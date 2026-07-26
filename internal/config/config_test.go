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
schedule:
  enabled: true
  interval: weekly
  command: apply
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
		Schedule: ScheduleConfig{Enabled: true, Interval: "weekly", Command: "apply"},
	}
	if got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadKeepsScheduleDefaultsWhenOmitted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yamlBody := "sources:\n  lynis:\n    enabled: false\n"
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := ScheduleConfig{Interval: "daily", Command: "check", Enabled: false}
	if got.Schedule != want {
		t.Fatalf("Load schedule = %+v, want defaults %+v", got.Schedule, want)
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

func TestGetRendersTypedFieldsAsStrings(t *testing.T) {
	cfg := Config{
		Sources: SourcesConfig{
			Lynis:    LynisConfig{Enabled: true, Quick: false, SkipUnchanged: true},
			Native:   NativeConfig{Enabled: false},
			Osquery:  OsqueryConfig{Enabled: true},
			OpenSCAP: OpenSCAPConfig{Enabled: true, Content: "/opt/ssg.xml", Profile: "cis"},
		},
	}
	cases := map[string]string{
		"sources.lynis.enabled":        "true",
		"sources.lynis.quick":          "false",
		"sources.lynis.skip_unchanged": "true",
		"sources.native.enabled":       "false",
		"sources.osquery.enabled":      "true",
		"sources.openscap.enabled":     "true",
		"sources.openscap.content":     "/opt/ssg.xml",
		"sources.openscap.profile":     "cis",
	}
	for key, want := range cases {
		got, err := Get(cfg, key)
		if err != nil {
			t.Fatalf("Get(%q): %v", key, err)
		}
		if got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestScheduleKeysAreGettableAndSettable(t *testing.T) {
	cfg, err := Set(Defaults(), "schedule.enabled", "true")
	if err != nil {
		t.Fatalf("Set schedule.enabled: %v", err)
	}
	cfg, err = Set(cfg, "schedule.interval", "weekly")
	if err != nil {
		t.Fatalf("Set schedule.interval: %v", err)
	}
	cfg, err = Set(cfg, "schedule.command", "apply")
	if err != nil {
		t.Fatalf("Set schedule.command: %v", err)
	}
	for key, want := range map[string]string{
		"schedule.enabled":  "true",
		"schedule.interval": "weekly",
		"schedule.command":  "apply",
	} {
		got, getErr := Get(cfg, key)
		if getErr != nil {
			t.Fatalf("Get(%q): %v", key, getErr)
		}
		if got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
	if _, err := Set(Defaults(), "schedule.enabled", "notabool"); err == nil {
		t.Error("expected an error setting schedule.enabled to a non-boolean")
	}
}

func TestGetRejectsUnknownKey(t *testing.T) {
	if _, err := Get(Defaults(), "sources.bogus.enabled"); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}

func TestKeysCoversEveryGettableField(t *testing.T) {
	// Keys() and Get must agree: every advertised key must resolve.
	for _, key := range Keys() {
		if _, err := Get(Defaults(), key); err != nil {
			t.Errorf("Keys() advertises %q but Get rejects it: %v", key, err)
		}
	}
}

func TestSetUpdatesOnlyTheTargetedField(t *testing.T) {
	got, err := Set(Defaults(), "sources.lynis.enabled", "false")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got.Sources.Lynis.Enabled {
		t.Error("expected sources.lynis.enabled to be set false")
	}
	// Every other field keeps its default.
	if !got.Sources.Native.Enabled || !got.Sources.Osquery.Enabled {
		t.Error("Set changed a field other than its target")
	}
}

func TestSetParsesStringField(t *testing.T) {
	got, err := Set(Defaults(), "sources.openscap.content", "/opt/ssg.xml")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got.Sources.OpenSCAP.Content != "/opt/ssg.xml" {
		t.Errorf("content = %q, want /opt/ssg.xml", got.Sources.OpenSCAP.Content)
	}
}

func TestSetRejectsUnknownKey(t *testing.T) {
	if _, err := Set(Defaults(), "sources.bogus.enabled", "true"); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}

func TestSetRejectsNonBooleanForBoolField(t *testing.T) {
	if _, err := Set(Defaults(), "sources.lynis.enabled", "yesplease"); err == nil {
		t.Fatal("expected an error for a non-boolean value on a bool field")
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	cfg, err := Set(Defaults(), "sources.lynis.enabled", "false")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if saveErr := Save(path, cfg); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != cfg {
		t.Fatalf("round-trip: Load = %+v, want %+v", got, cfg)
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

func TestRenderRoundTripsThroughLoad(t *testing.T) {
	cases := map[string]Config{
		"defaults": Defaults(),
		"all sources off, openscap on with content": {
			Sources: SourcesConfig{
				Lynis:    LynisConfig{Enabled: false, Quick: true, SkipUnchanged: true},
				Native:   NativeConfig{Enabled: false},
				Osquery:  OsqueryConfig{Enabled: false},
				OpenSCAP: OpenSCAPConfig{Enabled: true, Content: "/opt/ssg-content.xml", Profile: "xccdf_org.ssgproject.content_profile_cis"},
			},
		},
		"schedule enabled, weekly apply": {
			Sources:  Defaults().Sources,
			Schedule: ScheduleConfig{Enabled: true, Interval: "weekly", Command: "apply"},
		},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(Render(cfg)), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			got, err := Load(path)
			if err != nil {
				t.Fatalf("Load(Render(cfg)): %v", err)
			}
			if got != cfg {
				t.Fatalf("Load(Render(cfg)) = %+v, want %+v", got, cfg)
			}
		})
	}
}
