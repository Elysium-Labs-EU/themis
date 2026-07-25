package audit

import (
	"errors"
	"fmt"
	"sort"
)

// ErrSourceNotEnabled is the sentinel a Factory returns to signal that
// its source is not enabled for the given SourceConfig (e.g. OpenSCAP
// with no content path). Enabled detects it with errors.Is and leaves
// the source out of the built set rather than aborting; any other error
// aborts the build.
var ErrSourceNotEnabled = errors.New("audit: source not enabled for this config")

// SourceConfig carries the per-source options a factory needs to
// construct its Source, so the command layer can build the enabled set
// without knowing any source's internals. Zero value is the default
// full audit: lynis runs a full (non-quick) scan, openscap stays out
// (no content path), osquery uses its default state path.
//
// Fields are ordered pointer-bearing (strings) first so the struct packs
// tightly (govet fieldalignment).
type SourceConfig struct {
	// OpenSCAPContentPath is the SCAP/XCCDF datastream to evaluate. Empty
	// leaves OpenSCAP out of the enabled set entirely — its factory
	// returns ErrSourceNotEnabled.
	OpenSCAPContentPath string
	// OpenSCAPProfile is the XCCDF profile ID to evaluate; empty uses the
	// datastream's own default profile.
	OpenSCAPProfile string
	// OsqueryStatePath overrides the default apply-state path osquery's
	// drift detector reads. Empty uses the default (production).
	OsqueryStatePath string
	// LynisQuick runs lynis's lighter --quick profile instead of a full
	// audit.
	LynisQuick bool
	// LynisSkipUnchanged skips the lynis scan and reuses the last report
	// when nothing lynis cares about has changed since.
	LynisSkipUnchanged bool
}

// Factory constructs a Source from a SourceConfig. Returning
// ErrSourceNotEnabled means "not enabled for this config" (e.g. OpenSCAP
// with no content path) — Enabled skips the source rather than aborting.
type Factory func(SourceConfig) (Source, error)

// registration is one registered source: the factory that builds it, its
// name, and its position in the enabled order. Fields are ordered
// pointer-bearing (factory, name) first so the struct packs tightly
// (govet fieldalignment).
type registration struct {
	factory Factory
	name    string
	order   int
}

// registry is the process-wide set of registered source factories.
// Sources register themselves from an init() in their own package, so
// adding a source never touches cmd/check.go, cmd/plan.go, or
// cmd/api_check.go. Ordering is explicit (see order) rather than
// dependent on init() evaluation order across packages.
var registry []registration

// Register adds a source factory under name at the given order. order
// fixes the source's position in the slice Enabled returns (lower runs
// first); it is explicit so the enabled order does not depend on the
// unspecified order package init() functions run in. Register panics on a
// duplicate name — a name collision is a build-time wiring bug, not a
// runtime condition. Call it from an init() in the source's own package.
func Register(name string, order int, factory Factory) {
	for _, r := range registry {
		if r.name == name {
			panic(fmt.Sprintf("audit: source %q registered twice", name))
		}
	}
	registry = append(registry, registration{name: name, order: order, factory: factory})
}

// Enabled builds every registered source that opts in for cfg, in
// ascending order. A factory returning ErrSourceNotEnabled is skipped
// (the source is not enabled for this config); any other error aborts the
// whole build, wrapped with the source name. The result is ready to hand
// to Run.
func Enabled(cfg SourceConfig) ([]Source, error) {
	ordered := make([]registration, len(registry))
	copy(ordered, registry)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].order < ordered[j].order })

	sources := make([]Source, 0, len(ordered))
	for _, r := range ordered {
		s, err := r.factory(cfg)
		if errors.Is(err, ErrSourceNotEnabled) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("building source %q: %w", r.name, err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}
