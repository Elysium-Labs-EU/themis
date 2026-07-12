package cmd

import (
	"fmt"

	"codeberg.org/Elysium_Labs/themis/internal/lynis"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run a Lynis audit and list findings alongside themis-tracked checks",
	RunE: func(cmd *cobra.Command, _ []string) error {
		findings, err := lynis.Audit(cmd.Context())
		if err != nil {
			return fmt.Errorf("running lynis audit: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Lynis reported %d findings:\n", len(findings))
		for _, f := range findings {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s (%s) — %s\n", f.Kind, f.TestID, f.Severity, f.Description)
		}

		planned, err := resolveFixes()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nthemis-tracked checks:")
		for _, p := range planned {
			status := "satisfied"
			if !p.Satisfied {
				status = "NOT satisfied"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s — %s\n", status, p.TestID, p.Description)
		}
		return nil
	},
}
