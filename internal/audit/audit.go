// Package audit defines the Source interface that decouples themis from
// any single audit tool. Lynis is one Source among several; new sources
// (OpenSCAP, osquery, themis-native checks) plug in without touching
// cmd/check.go, cmd/plan.go, or cmd/api_check.go.
package audit

import "context"

// Finding is one suggestion or warning from an audit source.
type Finding struct {
	TestID      string
	Description string
	// Details is extra context for the finding, e.g. the config
	// directive and value that was flagged. Often "-".
	Details string
	// Solution is the source's own remediation hint (a command or
	// setting change). Often "-"; when present, the finding is
	// actionable even without a themis fix tracking it.
	Solution string
	// Kind is "suggestion" or "warning" for a normal audit finding, or
	// "drift" when a Source is reporting that a themis fix applied in a
	// prior run no longer holds (see internal/osquery). Drift findings
	// are routed separately by internal/checkreport rather than folded
	// into the generic finding list.
	Kind string
	// Source is the name of the Source that produced this finding, e.g.
	// "lynis".
	Source string
}

// Source is an audit tool themis can run to produce findings.
type Source interface {
	// Name identifies the source, e.g. "lynis". Used to tag findings and
	// in user-facing output.
	Name() string
	// Run executes the audit and returns its findings.
	Run(ctx context.Context) ([]Finding, error)
}

// Run executes every source and returns their findings concatenated. It
// stops and returns an error on the first source that fails.
func Run(ctx context.Context, sources []Source) ([]Finding, error) {
	var findings []Finding
	for _, s := range sources {
		f, err := s.Run(ctx)
		if err != nil {
			return nil, err
		}
		findings = append(findings, f...)
	}
	return findings, nil
}
