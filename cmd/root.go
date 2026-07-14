// Package cmd implements the themis CLI: check/plan/apply/rollback for
// Debian VPS hardening driven by Lynis findings.
package cmd

import (
	"codeberg.org/Elysium_Labs/themis/internal/buildinfo"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "themis",
	Short: "Lynis-driven Debian hardening CLI",
	Long: `themis wraps Lynis's audit findings with a check/plan/apply/rollback
workflow: it reads Lynis's report, maps flagged findings to concrete fixes,
and applies them idempotently with rollback metadata.`,
	// main.go renders errors itself (with UserError hints where available),
	// and a runtime failure like a missing lynis binary isn't a usage
	// mistake, so don't dump the flag usage block after it.
	SilenceErrors: true,
	SilenceUsage:  true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the themis version",
	Long:  `Print the current themis version, git commit hash, and build date.`,
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Println(buildinfo.Get())
	},
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
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
}
