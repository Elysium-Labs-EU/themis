package cmd

import (
	"github.com/Elysium-Labs-EU/themis/internal/buildinfo"
	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Manage the themis binary and check its version",
	Long:  `Manage the themis binary and runtime: check for updates, remove it, or print its version.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the themis version",
	Long:  `Print the current themis version, git commit hash, and build date.`,
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Println(buildinfo.Get())
	},
}

func init() {
	systemCmd.AddCommand(versionCmd)
	systemCmd.AddCommand(updateCmd)
	systemCmd.AddCommand(uninstallCmd)
}
