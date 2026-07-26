package cmd

import (
	"fmt"

	"github.com/Elysium-Labs-EU/themis/internal/config"
	"github.com/spf13/cobra"
)

// newConfigCmd is the `themis config` parent: non-interactive read/write
// of single operator-config values for scripted provisioning (Ansible,
// cloud-init), a counterpart to the interactive first-run setup. Every
// subcommand resolves keys through config's key namespace, so a typo is
// rejected rather than silently written.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and write single operator-config values non-interactively",
		Long: `Read and write themis's operator config one key at a time, without
an interactive prompt — for scripted or unattended provisioning.

Keys are dotted paths into the config file, e.g. sources.lynis.enabled.
Unknown keys are rejected so a typo surfaces immediately.`,
	}
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	return cmd
}

// newConfigPathCmd prints the resolved config file path — the same path
// get/set read and write — so a script can locate it without duplicating
// themis's THEMIS_CONFIG / root / home resolution.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), config.Path())
			return nil
		},
	}
}

// newConfigGetCmd prints one config value. Reads through Load, so a key
// the file omits reports its built-in default rather than an empty line.
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print one config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.Path())
			if err != nil {
				return fmt.Errorf("loading operator config: %w", err)
			}
			val, err := config.Get(cfg, args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

// newConfigSetCmd sets one config value and writes the file back. Load
// returns Defaults() for a missing file, so the first set creates a
// complete config from defaults with only the requested key changed; an
// unknown key or a value that doesn't fit the field's type fails before
// anything is written.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one config value and write the file",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			path := config.Path()
			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("loading operator config: %w", err)
			}
			cfg, err = config.Set(cfg, args[0], args[1])
			if err != nil {
				return err
			}
			return config.Save(path, cfg)
		},
	}
}

var configCmd = newConfigCmd()
