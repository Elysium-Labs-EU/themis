package cmd

import (
	"github.com/Elysium-Labs-EU/themis/internal/audit"
	"github.com/Elysium-Labs-EU/themis/internal/checkreport"
	"github.com/spf13/cobra"
)

type apiFix struct {
	TestID      string `json:"test_id"`
	Description string `json:"description"`
	Satisfied   bool   `json:"satisfied"`
}

type apiFinding struct {
	TestID      string   `json:"test_id"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Solution    string   `json:"solution"`
	Sources     []string `json:"sources"`
	Fixes       []apiFix `json:"fixes"`
	Actionable  bool     `json:"actionable"`
}

type apiDrift struct {
	TestID      string `json:"test_id"`
	Description string `json:"description"`
	Details     string `json:"details"`
}

type apiCheckResult struct {
	Findings    []apiFinding `json:"findings"`
	NativeFixes []apiFix     `json:"native_fixes"`
	Drift       []apiDrift   `json:"drift"`
}

var apiCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run an audit and return findings merged with themis fixes as JSON",
	Long: `Run every audit source (Lynis, themis-native checks) and return each
finding merged with any themis fix that tracks it, plus themis fixes that
have no matching finding. Unlike ` + "`themis check`" + `, nothing is filtered — the
"actionable" field marks findings that are noise (no themis fix, no
solution hint, not a warning) versus ones worth acting on.

Output schema (stdout, JSON):
  {
    "findings": [
      {
        "test_id":     string  -- source test ID (Lynis, or THEMIS-* for native checks)
        "kind":        string  -- "suggestion" or "warning"
        "description": string
        "solution":    string  -- the source's own remediation hint, "-" if none
        "sources":     []string -- audit source(s) that reported this finding, e.g. ["lynis"]
        "actionable":  bool    -- false if no themis fix, no solution, and not a warning
        "fixes": [
          { "test_id": string, "description": string, "satisfied": bool }
        ]
      }
    ],
    "native_fixes": [
      { "test_id": string, "description": string, "satisfied": bool }
    ],
    "drift": [
      { "test_id": string, "description": string, "details": string }
      -- fixes a prior ` + "`themis apply`" + ` confirmed satisfied that osquery now
         reports as no longer holding; kept separate from "findings" since
         it's a regression, not a fresh suggestion
    ]
  }

Error schema (stderr, JSON):
  { "error": "string" }

Exit codes:
  0  success
  1  error`,
	Example: `  themis api check
  themis api check | jq '.findings[] | select(.actionable)'
  themis api check | jq '[.findings[].fixes[] | select(.satisfied == false)]'`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		quick, _ := cmd.Flags().GetBool("quick")
		findings, err := audit.Run(cmd.Context(), sources(quick))
		if err != nil {
			return writeJSONErr(cmd, err)
		}
		fixes, err := resolveCheckFixes()
		if err != nil {
			return writeJSONErr(cmd, err)
		}
		report := checkreport.Build(findings, fixes)

		result := apiCheckResult{
			Findings:    make([]apiFinding, 0, len(report.Findings)),
			NativeFixes: make([]apiFix, 0, len(report.Native)),
			Drift:       make([]apiDrift, 0, len(report.Drift)),
		}
		for _, f := range report.Findings {
			fs := make([]apiFix, 0, len(f.Fixes))
			for _, fx := range f.Fixes {
				fs = append(fs, apiFix{TestID: fx.TestID, Description: fx.Description, Satisfied: fx.Satisfied})
			}
			result.Findings = append(result.Findings, apiFinding{
				TestID:      f.TestID,
				Kind:        f.Kind,
				Description: f.Description,
				Solution:    f.Solution,
				Sources:     f.Sources,
				Actionable:  f.Actionable,
				Fixes:       fs,
			})
		}
		for _, f := range report.Native {
			result.NativeFixes = append(result.NativeFixes, apiFix{TestID: f.TestID, Description: f.Description, Satisfied: f.Satisfied})
		}
		for _, d := range report.Drift {
			result.Drift = append(result.Drift, apiDrift{TestID: d.TestID, Description: d.Description, Details: d.Details})
		}

		return writeJSON(cmd, result)
	},
}
