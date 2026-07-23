package lynis

import (
	"context"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
)

// Source runs Lynis as a pluggable audit.Source.
type Source struct {
	opts Options
}

// NewSource returns a Lynis audit.Source. opts.Quick controls whether the
// scan uses lynis's lighter --quick profile or a full audit (default);
// opts.SkipIfUnchanged controls whether an unchanged host skips the scan
// and reuses the last report.
func NewSource(opts Options) Source { return Source{opts: opts} }

// Name identifies this source as "lynis".
func (Source) Name() string { return "lynis" }

// Run audits the system with Lynis and returns its findings as
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
			Source:      "lynis",
		})
	}
	return out, nil
}
