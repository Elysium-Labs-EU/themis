package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// errAPICommandFailed is returned by api subcommands after writing a JSON
// error to stderr, so RunE reports a non-zero exit without cobra also
// printing the error itself (SilenceErrors handles that).
var errAPICommandFailed = errors.New("api command failed")

func writeJSON(cmd *cobra.Command, v any) error {
	out, err := json.Marshal(v)
	if err != nil {
		return writeJSONErr(cmd, err)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func writeJSONErr(cmd *cobra.Command, err error) error {
	out, _ := json.Marshal(map[string]string{"error": err.Error()})
	cmd.PrintErrln(string(out))
	return errAPICommandFailed
}

var apiCmd = &cobra.Command{
	Use:           "api",
	Short:         "Machine-readable JSON interface",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	apiCheckCmd.Flags().Bool("quick", false, "run lynis's lighter --quick profile instead of a full audit")
	apiCheckCmd.Flags().String("scap-content", "", "path to a SCAP/XCCDF datastream (e.g. oscap-ssg content); also runs OpenSCAP when set")
	apiCheckCmd.Flags().String("scap-profile", "", "XCCDF profile ID to evaluate (default: the datastream's own default profile)")
	apiCmd.AddCommand(apiCheckCmd)
}
