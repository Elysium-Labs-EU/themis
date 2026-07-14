// Package cmd implements the themis CLI: check/plan/apply/rollback for
// automated Debian hardening.
package cmd

import (
	"fmt"

	"codeberg.org/Elysium_Labs/themis/internal/buildinfo"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "themis",
	Short: "Automated Debian hardening CLI",
	Long: fmt.Sprintf(`themis %s

themis wraps audit findings with a check/plan/apply/rollback workflow:
it maps flagged findings to concrete fixes, applies them idempotently,
and saves rollback metadata on every run.`, buildinfo.GetVersionOnly()),
	// main.go renders errors itself (with UserError hints where available),
	// and a runtime failure like a missing lynis binary isn't a usage
	// mistake, so don't dump the flag usage block after it.
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute runs the themis CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(systemCmd)
}
