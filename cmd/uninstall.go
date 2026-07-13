package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"codeberg.org/Elysium_Labs/themis/internal/state"
	"codeberg.org/Elysium_Labs/themis/internal/ui"
	"github.com/spf13/cobra"
)

// runUninstall implements `themis uninstall` against explicit paths so it
// can be exercised in tests without touching the real installed binary or
// os.Executable() (which, under `go test`, is the test binary itself).
func runUninstall(in io.Reader, out io.Writer, exePath, stateDir string, yes, purge bool) error {
	if !yes && !ui.Confirm(in, out, fmt.Sprintf("Remove themis (%s)?", exePath), false) {
		fmt.Fprintln(out, "Cancelled.")
		return nil
	}

	if err := os.Remove(exePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", exePath, err)
	}
	fmt.Fprintf(out, "%s removed %s\n", ui.LabelSuccess.Render("✓"), exePath)

	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return nil
	}

	removeState := purge
	if !removeState && !yes {
		removeState = ui.Confirm(in, out, fmt.Sprintf("Also remove themis state data (%s)?", stateDir), false)
	}

	if removeState {
		if err := os.RemoveAll(stateDir); err != nil {
			return fmt.Errorf("removing %s: %w", stateDir, err)
		}
		fmt.Fprintf(out, "%s removed %s\n", ui.LabelSuccess.Render("✓"), stateDir)
	} else {
		fmt.Fprintf(out, "%s state data left in place — remove manually: %s\n",
			ui.TextMuted.Render("i"), ui.TextCommand.Render("rm -rf "+stateDir))
	}
	return nil
}

func newUninstallCmd() *cobra.Command {
	var yes bool
	var purge bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the themis binary",
		Long: `Remove the themis binary.

By default the state directory (rollback metadata from ` + "`themis apply`" + `)
is left in place and a manual cleanup hint is printed. Pass --purge to
remove it too.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exePath, err := currentBinaryPath()
			if err != nil {
				return err
			}
			return runUninstall(cmd.InOrStdin(), cmd.OutOrStdout(), exePath, filepath.Dir(state.DefaultPath), yes, purge)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the binary-removal confirmation prompt")
	cmd.Flags().BoolVar(&purge, "purge", false, "also remove state data (rollback metadata) without prompting")
	return cmd
}

var uninstallCmd = newUninstallCmd()
