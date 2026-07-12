package cmd

import (
	"fmt"
	"sort"

	"codeberg.org/Elysium_Labs/themis/internal/fix"
	"github.com/spf13/cobra"
)

// PlannedFix is one registry Fix and whether it is currently satisfied.
type PlannedFix struct {
	TestID      string
	Description string
	Satisfied   bool
}

// resolveFixes checks every registered Fix and returns the result in a
// stable, sorted order.
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
		planned = append(planned, PlannedFix{TestID: id, Description: f.Description, Satisfied: satisfied})
	}
	return planned, nil
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show which registered fixes would be applied",
	RunE: func(cmd *cobra.Command, _ []string) error {
		planned, err := resolveFixes()
		if err != nil {
			return err
		}
		toApply := 0
		for _, p := range planned {
			if p.Satisfied {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [ok]      %s — %s\n", p.TestID, p.Description)
				continue
			}
			toApply++
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [+apply]  %s — %s\n", p.TestID, p.Description)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d fix(es) would be applied, %d already satisfied.\n", toApply, len(planned)-toApply)
		return nil
	},
}
