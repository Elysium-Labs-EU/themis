// Package config loads themis's persisted operator configuration: a YAML
// file that sets defaults for which audit sources run and their
// per-source options, so an operator doesn't have to repeat CLI flags on
// every invocation. Precedence is built-in defaults < config file < CLI
// flags — this package only resolves the first two; flag overrides are
// applied by the command layer on top of what Load returns.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EnvVar is the environment variable that overrides the operator config
// path, for tests and non-standard installs — mirrors the seam eos and
// argus use for their own config paths.
const EnvVar = "THEMIS_CONFIG"

// rootConfigPath is where the operator config lives for a root install.
const rootConfigPath = "/etc/themis/config.yaml"

// userConfigRelPath is where the operator config falls back to for a
// non-root install, relative to the user's home directory.
const userConfigRelPath = ".themis/config.yaml"

// Config is themis's persisted operator configuration.
type Config struct {
	Sources SourcesConfig `yaml:"sources"`
}

// SourcesConfig is the enabled/options state of every audit source
// themis knows about, keyed by source name to match the audit registry.
type SourcesConfig struct {
	OpenSCAP OpenSCAPConfig `yaml:"openscap"`
	Lynis    LynisConfig    `yaml:"lynis"`
	Native   NativeConfig   `yaml:"native"`
	Osquery  OsqueryConfig  `yaml:"osquery"`
}

// LynisConfig is the operator-configurable state of the lynis source.
type LynisConfig struct {
	Enabled       bool `yaml:"enabled"`
	Quick         bool `yaml:"quick"`
	SkipUnchanged bool `yaml:"skip_unchanged"`
}

// NativeConfig is the operator-configurable state of the themis-native
// source.
type NativeConfig struct {
	Enabled bool `yaml:"enabled"`
}

// OsqueryConfig is the operator-configurable state of the osquery
// drift-detection source.
type OsqueryConfig struct {
	Enabled bool `yaml:"enabled"`
}

// OpenSCAPConfig is the operator-configurable state of the openscap
// source.
type OpenSCAPConfig struct {
	Content string `yaml:"content"`
	Profile string `yaml:"profile"`
	Enabled bool   `yaml:"enabled"`
}

// Defaults is themis's built-in configuration before any config file or
// CLI flag is applied: lynis, themis-native, and osquery all run a full
// audit; openscap stays out (no content configured). This is the bottom
// of the defaults < file < flags precedence chain.
func Defaults() Config {
	return Config{
		Sources: SourcesConfig{
			Lynis:    LynisConfig{Enabled: true},
			Native:   NativeConfig{Enabled: true},
			Osquery:  OsqueryConfig{Enabled: true},
			OpenSCAP: OpenSCAPConfig{Enabled: false},
		},
	}
}

// Path resolves the operator config file location: THEMIS_CONFIG if set,
// else /etc/themis/config.yaml for a root install, else
// ~/.themis/config.yaml. Falls back to the root path if the home
// directory can't be resolved — Load then finds nothing there and
// returns Defaults(), same as any other missing file.
func Path() string {
	if p := os.Getenv(EnvVar); p != "" {
		return p
	}
	if os.Geteuid() == 0 {
		return rootConfigPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return rootConfigPath
	}
	return filepath.Join(home, userConfigRelPath)
}

// Load reads and parses the config file at path, merging it on top of
// Defaults(): keys the file sets override the default, keys it omits
// keep the default. A missing file is not an error — it means "all
// defaults", same as an empty file.
func Load(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path) //nolint:gosec // path comes from Path(), a documented test/deployment seam (THEMIS_CONFIG), not arbitrary user input
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
