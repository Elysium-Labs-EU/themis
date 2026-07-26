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
	"sort"
	"strconv"
	"strings"

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

// Config is themis's persisted operator configuration. (Schedule leads
// Sources only to satisfy govet's fieldalignment — the trailing bools in
// SourcesConfig otherwise widen the GC pointer-scan region; field order
// carries no semantic weight.)
type Config struct {
	Schedule ScheduleConfig `yaml:"schedule"`
	Sources  SourcesConfig  `yaml:"sources"`
}

// ScheduleConfig is the operator-configurable state of the OS-native
// recurring scan themis installs via `themis schedule enable`. Interval
// is "daily", "weekly", or a raw systemd OnCalendar expression (only
// daily/weekly translate to launchd/cron); Command is "check" or "apply".
// Enabled records intent — it does not itself install the unit; `themis
// schedule enable` does that and `disable` removes it.
type ScheduleConfig struct {
	Interval string `yaml:"interval"`
	Command  string `yaml:"command"`
	Enabled  bool   `yaml:"enabled"`
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
		Schedule: ScheduleConfig{
			Interval: "daily",
			Command:  "check",
			Enabled:  false,
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

// yamlBool renders b as a YAML boolean literal ("true"/"false") — the
// same tokens Load's yaml.Unmarshal parses back into a bool.
func yamlBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// yamlString renders s as a double-quoted YAML scalar, so an empty value
// serializes as `""` (a visible, self-documenting placeholder the wizard
// can leave for the operator to fill in) rather than a bare newline.
// Content and profile paths in practice never contain a double quote or
// backslash; guard anyway so a pathological value can't break the file.
func yamlString(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// Render produces the self-documenting YAML text for cfg: a leading
// comment block explaining precedence plus every key Load reads, each
// with an inline comment. This is themis's config documentation surface
// — `themis init` writes it verbatim, so keep the keys and comments here
// in sync with the yaml tags Load unmarshals into. The output round-trips:
// Load(Render(cfg)) == cfg. Pure — no I/O.
func Render(cfg Config) string { //nolint:gocritic // STYLE.md mandates value semantics for config/data; Config crossed hugeParam's 80-byte bound only when the schedule block was added — kept by value, not pointer-converted
	s := cfg.Sources
	sc := cfg.Schedule
	return fmt.Sprintf(`# themis operator configuration
#
# Sets defaults for which audit sources run and their per-source options,
# so you don't have to repeat CLI flags on every invocation. Precedence is
# built-in defaults < this file < CLI flags: a flag always wins where it is
# explicitly passed. Omit any key to keep its built-in default.
#
# Regenerate this file at any time with: themis init
sources:
  # Lynis: the third-party system auditor themis wraps. Always a themis
  # dependency, so it runs by default.
  lynis:
    enabled: %s
    quick: %s          # run lynis's lighter --quick profile instead of a full audit
    skip_unchanged: %s # reuse the last report when nothing lynis cares about (config, package list) changed
  # themis-native: themis's own built-in checks. No external dependency.
  native:
    enabled: %s
  # osquery: drift detection against fixes a prior 'themis apply' recorded.
  # No-ops safely when osqueryi isn't installed or there is no prior state.
  osquery:
    enabled: %s
  # OpenSCAP: evaluates a SCAP/XCCDF datastream. Off by default — unlike
  # lynis it is not a themis dependency, and most hosts have no SCAP content
  # installed. Set content to a datastream path and enabled: true to run it.
  openscap:
    enabled: %s
    content: %s # path to a SCAP/XCCDF datastream (e.g. ssg-debian content); required when enabled
    profile: %s # XCCDF profile ID to evaluate; empty uses the datastream's own default profile
# schedule: an OS-native recurring scan (systemd timer, launchd agent, or
# cron entry). This block records intent and the parameters; installing or
# removing the unit is done with 'themis schedule enable' / 'disable'.
schedule:
  enabled: %s
  interval: %s # daily | weekly | a raw systemd OnCalendar expression (daily/weekly also map to launchd/cron)
  command: %s  # check | apply — the themis subcommand each scheduled run invokes
`,
		yamlBool(s.Lynis.Enabled),
		yamlBool(s.Lynis.Quick),
		yamlBool(s.Lynis.SkipUnchanged),
		yamlBool(s.Native.Enabled),
		yamlBool(s.Osquery.Enabled),
		yamlBool(s.OpenSCAP.Enabled),
		yamlString(s.OpenSCAP.Content),
		yamlString(s.OpenSCAP.Profile),
		yamlBool(sc.Enabled),
		yamlString(sc.Interval),
		yamlString(sc.Command),
	)
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

// dirPerm/filePerm are the modes Save creates the config dir and file
// with: world-readable, since the operator config holds no secrets (just
// which sources run and their options) and other tooling stats it.
const (
	dirPerm  os.FileMode = 0o755
	filePerm os.FileMode = 0o644
)

// Save writes cfg to path as YAML, creating the parent directory if it
// doesn't exist. It marshals the whole Config, so a file created from
// Defaults() by the first Set lands complete rather than sparse. The
// effect boundary for the pure Get/Set below — those never touch disk.
func Save(path string, cfg Config) error { //nolint:gocritic // STYLE.md mandates value semantics for config/data; Config crossed hugeParam's 80-byte bound only when the schedule block was added — kept by value, not pointer-converted
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("creating config dir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// keyField binds one dotted config key (e.g. "sources.lynis.enabled") to
// how it is read from and written into a Config. get renders the current
// value as the string the CLI prints; set parses a string into the typed
// field and returns the updated Config, or an error if the value doesn't
// fit the field's type. Both are pure — no I/O, value in / value out.
type keyField struct {
	get func(Config) string
	set func(Config, string) (Config, error)
}

// keyRegistry is the single source of truth for the config key namespace:
// every operator-settable key, its getter, and its typed setter. `themis
// config get/set` and any future wizard resolve keys through this one map,
// so a new field is one entry here rather than a parallel edit in each
// caller. A key absent from this map is, by definition, unknown — that is
// what makes `set sources.bogus.enabled` fail fast instead of silently
// writing a field nothing reads.
func keyRegistry() map[string]keyField {
	return map[string]keyField{
		"sources.lynis.enabled": {
			get: func(c Config) string { return strconv.FormatBool(c.Sources.Lynis.Enabled) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Sources.Lynis.Enabled = b
				return c, nil
			},
		},
		"sources.lynis.quick": {
			get: func(c Config) string { return strconv.FormatBool(c.Sources.Lynis.Quick) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Sources.Lynis.Quick = b
				return c, nil
			},
		},
		"sources.lynis.skip_unchanged": {
			get: func(c Config) string { return strconv.FormatBool(c.Sources.Lynis.SkipUnchanged) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Sources.Lynis.SkipUnchanged = b
				return c, nil
			},
		},
		"sources.native.enabled": {
			get: func(c Config) string { return strconv.FormatBool(c.Sources.Native.Enabled) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Sources.Native.Enabled = b
				return c, nil
			},
		},
		"sources.osquery.enabled": {
			get: func(c Config) string { return strconv.FormatBool(c.Sources.Osquery.Enabled) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Sources.Osquery.Enabled = b
				return c, nil
			},
		},
		"sources.openscap.enabled": {
			get: func(c Config) string { return strconv.FormatBool(c.Sources.OpenSCAP.Enabled) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Sources.OpenSCAP.Enabled = b
				return c, nil
			},
		},
		"sources.openscap.content": {
			get: func(c Config) string { return c.Sources.OpenSCAP.Content },
			set: func(c Config, v string) (Config, error) {
				c.Sources.OpenSCAP.Content = v
				return c, nil
			},
		},
		"sources.openscap.profile": {
			get: func(c Config) string { return c.Sources.OpenSCAP.Profile },
			set: func(c Config, v string) (Config, error) {
				c.Sources.OpenSCAP.Profile = v
				return c, nil
			},
		},
		"schedule.enabled": {
			get: func(c Config) string { return strconv.FormatBool(c.Schedule.Enabled) },
			set: func(c Config, v string) (Config, error) {
				b, err := parseBool(v)
				if err != nil {
					return Config{}, err
				}
				c.Schedule.Enabled = b
				return c, nil
			},
		},
		"schedule.interval": {
			get: func(c Config) string { return c.Schedule.Interval },
			set: func(c Config, v string) (Config, error) {
				c.Schedule.Interval = v
				return c, nil
			},
		},
		"schedule.command": {
			get: func(c Config) string { return c.Schedule.Command },
			set: func(c Config, v string) (Config, error) {
				c.Schedule.Command = v
				return c, nil
			},
		},
	}
}

// Keys returns every known config key, sorted — for error messages and
// help text that enumerate the namespace.
func Keys() []string {
	reg := keyRegistry()
	keys := make([]string, 0, len(reg))
	for k := range reg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Get returns the current value of key in cfg as a string, or an error if
// key is not part of the namespace.
func Get(cfg Config, key string) (string, error) { //nolint:gocritic // STYLE.md mandates value semantics for config/data; Config crossed hugeParam's 80-byte bound only when the schedule block was added — kept by value, not pointer-converted
	f, ok := keyRegistry()[key]
	if !ok {
		return "", unknownKeyError(key)
	}
	return f.get(cfg), nil
}

// Set returns cfg with key updated to value, or an error if key is
// unknown or value doesn't parse for the field's type. Pure — the caller
// persists the result with Save.
func Set(cfg Config, key, value string) (Config, error) { //nolint:gocritic // STYLE.md mandates value semantics for config/data; Config crossed hugeParam's 80-byte bound only when the schedule block was added — kept by value, not pointer-converted
	f, ok := keyRegistry()[key]
	if !ok {
		return Config{}, unknownKeyError(key)
	}
	return f.set(cfg, value)
}

// parseBool accepts the same spellings strconv.ParseBool does (true/false,
// 1/0, t/f), rewrapping the error so a bad value names what was expected.
func parseBool(v string) (bool, error) {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid boolean %q: want true or false", v)
	}
	return b, nil
}

// unknownKeyError reports an unrecognized config key, listing the valid
// keys so a typo is corrected on the spot rather than guessed at.
func unknownKeyError(key string) error {
	return fmt.Errorf("unknown config key %q; valid keys: %s", key, strings.Join(Keys(), ", "))
}
