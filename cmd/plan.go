package cmd

import (
	"fmt"
	"sort"

	"github.com/Elysium-Labs-EU/themis/internal/checkreport"
	"github.com/Elysium-Labs-EU/themis/internal/fix"
	"github.com/spf13/cobra"
)

// PlannedFix is one registry Fix and whether it is currently satisfied.
type PlannedFix struct {
	TestID      string
	Description string
	// WarnMessage mirrors cmd/apply.go's own Fix.Warn check, so plan reports
	// the same "skipped, no mutation" outcome apply will actually produce
	// for an unsatisfied fix apply would warn on instead of applying.
	WarnMessage string
	Satisfied   bool
	Warned      bool
}

// resolveFixes checks every registered Fix and returns the result in a
// stable, sorted order. For an unsatisfied fix with a Warn func, Warn is
// also evaluated here so the result matches what cmd/apply.go's runApply
// will actually do, rather than just what Check reports.
func resolveFixes() ([]PlannedFix, error) {
	ids := make([]string, 0, len(fix.Registry))
	for id := range fix.Registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	planned := make([]PlannedFix, 0, len(ids))
	for _, id := range ids {
		f := fix.Registry[id]
		satisfied, err := f.Check()
		if err != nil {
			return nil, fmt.Errorf("checking %s: %w", id, err)
		}
		p := PlannedFix{TestID: id, Description: f.Description, Satisfied: satisfied}
		if !satisfied && f.Warn != nil {
			msg, detected, warnErr := f.Warn()
			if warnErr != nil {
				return nil, fmt.Errorf("checking %s for warnings: %w", id, warnErr)
			}
			if detected {
				p.Warned = true
				p.WarnMessage = msg
			}
		}
		planned = append(planned, p)
	}
	return planned, nil
}

// resolveCheckFixes runs resolveFixes and resolves each result against
// the Lynis test ID it addresses, for merging with lynis.Audit output.
func resolveCheckFixes() ([]checkreport.Fix, error) {
	planned, err := resolveFixes()
	if err != nil {
		return nil, err
	}
	fixes := make([]checkreport.Fix, 0, len(planned))
	for _, p := range planned {
		fixes = append(fixes, checkreport.Fix{
			TestID:      p.TestID,
			LynisID:     fix.Registry[p.TestID].LynisTestID(),
			Description: p.Description,
			Satisfied:   p.Satisfied,
		})
	}
	return fixes, nil
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show which registered fixes would be applied",
	RunE: func(cmd *cobra.Command, _ []string) error {
		planned, err := resolveFixes()
		if err != nil {
			return err
		}
		toApply, warned := 0, 0
		for _, p := range planned {
			switch {
			case p.Satisfied:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [ok]      %s — %s\n", p.TestID, p.Description)
			case p.Warned:
				warned++
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [warn]    %s — %s\n", p.TestID, p.WarnMessage)
			default:
				toApply++
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [+apply]  %s — %s\n", p.TestID, p.Description)
			}
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d fix(es) would be applied, %d already satisfied, %d would be skipped with a warning.\n",
			toApply, len(planned)-toApply-warned, warned)
		return nil
	},
}
