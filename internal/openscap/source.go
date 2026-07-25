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
