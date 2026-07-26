package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Elysium-Labs-EU/themis/internal/config"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

// promptConfig walks the operator through every value themis init writes,
// seeding each prompt with def's value (the built-in defaults) as the
// bracketed default so a bare Enter keeps it. Per-source options that only
// matter when the source is enabled (lynis quick/skip-unchanged, openscap
// content/profile) are asked only when the operator enables that source.
// I/O boundary: it reads from reader and writes prompts to out; the merge
// itself is a straight-line build of a config.Config value.
func promptConfig(reader *bufio.Reader, out io.Writer, def config.Config) config.Config { //nolint:gocritic // STYLE.md mandates value semantics for config/data; config.Config crossed hugeParam's 80-byte bound only when the schedule block was added — kept by value, not pointer-converted
	cfg := def

	cfg.Sources.Lynis.Enabled = ui.Confirm(reader, out, "Enable the Lynis source?", def.Sources.Lynis.Enabled)
	if cfg.Sources.Lynis.Enabled {
		cfg.Sources.Lynis.Quick = ui.Confirm(reader, out, "  Lynis: run the lighter --quick profile?", def.Sources.Lynis.Quick)
		cfg.Sources.Lynis.SkipUnchanged = ui.Confirm(reader, out, "  Lynis: skip the scan and reuse the last report when nothing changed?", def.Sources.Lynis.SkipUnchanged)
	}

	cfg.Sources.Native.Enabled = ui.Confirm(reader, out, "Enable the themis-native source?", def.Sources.Native.Enabled)
	cfg.Sources.Osquery.Enabled = ui.Confirm(reader, out, "Enable the osquery drift-detection source?", def.Sources.Osquery.Enabled)

	cfg.Sources.OpenSCAP.Enabled = ui.Confirm(reader, out, "Enable the OpenSCAP source?", def.Sources.OpenSCAP.Enabled)
	if cfg.Sources.OpenSCAP.Enabled {
		cfg.Sources.OpenSCAP.Content = ui.Prompt(reader, out, "  OpenSCAP: path to a SCAP/XCCDF datastream", def.Sources.OpenSCAP.Content)
		cfg.Sources.OpenSCAP.Profile = ui.Prompt(reader, out, "  OpenSCAP: XCCDF profile ID (blank = datastream default)", def.Sources.OpenSCAP.Profile)
	}

	return cfg
}

// writeConfig writes content to path, creating the parent directory
// (0o700 — the config can name a SCAP content path, nothing secret, but
// /etc/themis and ~/.themis are themis-owned so keep them tight) if it
// doesn't exist. Effect at the boundary; the content it writes is built
// by config.Render, a pure function.
func writeConfig(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// runInit implements `themis init` against an explicit path so it can be
// exercised in tests without touching the real resolved config location.
// With yes set it writes the built-in defaults unprompted and overwrites
// any existing file; without it, it prompts for every value and refuses to
// clobber an existing file unless the operator confirms the overwrite.
func runInit(in io.Reader, out io.Writer, path string, yes bool) error {
	reader := bufio.NewReader(in)

	if _, err := os.Stat(path); err == nil && !yes {
		if !ui.Confirm(reader, out, fmt.Sprintf("%s already exists — overwrite?", path), false) {
			_, _ = fmt.Fprintln(out, "Canceled.")
			return nil
		}
	}

	cfg := config.Defaults()
	if !yes {
		cfg = promptConfig(reader, out, cfg)
	}

	if err := writeConfig(path, config.Render(cfg)); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "%s wrote %s\n", ui.LabelSuccess.Render("✓"), path)
	_, _ = fmt.Fprintf(out, "  run %s to audit with it\n", ui.TextCommand.Render("themis check"))
	return nil
}

func newInitCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a commented config.yaml",
		Long: `Interactively scaffold themis's operator config file.

The wizard prompts for which audit sources to enable and the per-source
options that matter, then writes a self-documenting, commented config.yaml
to the resolved config path (THEMIS_CONFIG, else /etc/themis/config.yaml for
root, else ~/.themis/config.yaml).

Pass --yes to write the built-in defaults without prompting (and overwrite
any existing file); otherwise an existing file is left untouched unless you
confirm the overwrite.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd.InOrStdin(), cmd.OutOrStdout(), config.Path(), yes)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "write the built-in defaults without prompting, overwriting any existing file")
	return cmd
}

var initCmd = newInitCmd()
