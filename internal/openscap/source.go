package openscap

import (
	"context"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
)

// Source runs OpenSCAP (`oscap xccdf eval`) as a pluggable audit.Source.
type Source struct {
	opts Options
}

// NewSource returns an OpenSCAP audit.Source. opts.ContentPath must point
// at a SCAP/XCCDF datastream (e.g. oscap-ssg content); Audit errors
// without it.
func NewSource(opts Options) Source { return Source{opts: opts} }

// init registers openscap in the audit source registry so the command
// layer builds it by name via audit.Enabled. order 40 keeps it last,
// matching the historical order. Unlike the other sources, openscap is
// only enabled when a content path is configured: with none it returns
// audit.ErrSourceNotEnabled, so Enabled leaves it out entirely (most
// hosts have no SCAP content installed, and — unlike lynis — it is not a
// themis dependency).
func init() {
	audit.Register("openscap", 40, func(cfg audit.SourceConfig) (audit.Source, error) {
		if cfg.OpenSCAPContentPath == "" {
			return nil, audit.ErrSourceNotEnabled
		}
		return NewSource(Options{ContentPath: cfg.OpenSCAPContentPath, Profile: cfg.OpenSCAPProfile}), nil
	})
}

// Name identifies this source as "openscap".
func (Source) Name() string { return "openscap" }

// Run audits the system with OpenSCAP and returns its findings as
// audit.Finding.
func (s Source) Run(ctx context.Context) ([]audit.Finding, error) {
	findings, err := Audit(ctx, s.opts)
	if err != nil {
		return nil, err
	}
	out := make([]audit.Finding, 0, len(findings))
	for _, f := range findings {
		out = append(out, audit.Finding{
			TestID:      f.TestID,
			Description: f.Description,
			Details:     f.Details,
			Solution:    f.Solution,
			Kind:        f.Kind,
			Source:      "openscap",
		})
	}
	return out, nil
}
