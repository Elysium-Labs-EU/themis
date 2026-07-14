package lynis

import (
	"context"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
)

// Source runs Lynis as a pluggable audit.Source.
type Source struct{}

// NewSource returns a Lynis audit.Source.
func NewSource() Source { return Source{} }

// Name identifies this source as "lynis".
func (Source) Name() string { return "lynis" }

// Run audits the system with Lynis and returns its findings as
// audit.Finding.
func (Source) Run(ctx context.Context) ([]audit.Finding, error) {
	findings, err := Audit(ctx)
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
