// Package cmd implements the themis CLI: check/plan/apply/rollback for
// Debian VPS hardening driven by Lynis findings.
package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "themis",
	Short: "Lynis-driven Debian hardening CLI",
	Long: `themis wraps Lynis's audit findings with a check/plan/apply/rollback
workflow: it reads Lynis's report, maps flagged findings to concrete fixes,
and applies them idempotently with rollback metadata.`,
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
}
