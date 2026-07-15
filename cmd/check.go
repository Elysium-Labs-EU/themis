package cmd

import (
	"fmt"
	"io"
	"strings"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
	"codeberg.org/Elysium_Labs/themis/internal/checkreport"
	"codeberg.org/Elysium_Labs/themis/internal/lynis"
	"codeberg.org/Elysium_Labs/themis/internal/native"
	"codeberg.org/Elysium_Labs/themis/internal/ui"
	"github.com/spf13/cobra"
)

// sources lists every audit source themis runs when auditing the system
// (themis check, themis api check). quick, when true, runs lynis with its
// lighter --quick profile instead of a full audit.
func sources(quick bool) []audit.Source {
	return []audit.Source{lynis.NewSource(lynis.Options{Quick: quick}), native.NewSource()}
}

func newCheckCmd() *cobra.Command {
	var showAll bool
	var quick bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run an audit and list actionable findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var findings []audit.Finding
			err := ui.WithSpinner("Running audit...", func() error {
				var err error
				findings, err = audit.Run(cmd.Context(), sources(quick))
				return err
			})
			if err != nil {
				return err
			}
			fixes, err := resolveCheckFixes()
			if err != nil {
				return err
			}
			report := checkreport.Build(findings, fixes)
			printCheckReport(cmd, report, showAll)
			return nil
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "also show findings with no themis fix and no source solution hint")
	cmd.Flags().BoolVar(&quick, "quick", false, "run lynis's lighter --quick profile instead of a full audit")
	return cmd
}

var checkCmd = newCheckCmd()

func fixSummary(fixes []checkreport.Fix) string {
	parts := make([]string, 0, len(fixes))
	for _, f := range fixes {
		icon := ui.LabelWarning.Render("○ apply")
		if f.Satisfied {
			icon = ui.LabelSuccess.Render("✓ fixed")
		}
		parts = append(parts, f.TestID+" "+icon)
	}
	return strings.Join(parts, ", ")
}

func printFindingBlock(out io.Writer, f *checkreport.Finding) {
	kind := ui.TextMuted.Render(f.Kind)
	if f.Kind == "warning" {
		kind = ui.LabelWarning.Render(f.Kind)
	}
	_, _ = fmt.Fprintf(out, "%s %s\n", ui.TextBold.Render(f.TestID), kind)
	_, _ = fmt.Fprintf(out, "  %s\n", f.Description)
	if f.Solution != "" && f.Solution != "-" {
		_, _ = fmt.Fprintf(out, "  %s %s\n", ui.TextMuted.Render("solution:"), f.Solution)
	}
	if len(f.Fixes) > 0 {
		_, _ = fmt.Fprintf(out, "  %s %s\n", ui.TextMuted.Render("themis fix:"), fixSummary(f.Fixes))
	}
}

func printCheckReport(cmd *cobra.Command, report checkreport.Report, showAll bool) {
	out := cmd.OutOrStdout()

	shown := make([]checkreport.Finding, 0, len(report.Findings))
	deemphasized := make([]checkreport.Finding, 0, len(report.Findings))
	for _, f := range report.Findings {
		if !f.Actionable && !showAll {
			deemphasized = append(deemphasized, f)
			continue
		}
		shown = append(shown, f)
	}

	_, _ = fmt.Fprintf(out, "%s audit reported %d finding(s)\n\n", ui.LabelInfo.Render("i"), len(report.Findings))

	for i := range shown {
		if i > 0 {
			_, _ = fmt.Fprintln(out)
		}
		printFindingBlock(out, &shown[i])
	}

	if len(deemphasized) > 0 {
		_, _ = fmt.Fprintf(out, "\n%s %d finding(s) themis can't act on directly (no fix, no solution hint):\n",
			ui.TextMuted.Render("i"), len(deemphasized))
		for _, f := range deemphasized {
			_, _ = fmt.Fprintf(out, "  %s\n", ui.TextMuted.Render(f.TestID+" — "+f.Description))
		}
		_, _ = fmt.Fprintf(out, "  run %s for full details\n", ui.TextCommand.Render("themis check --all"))
	}

	if len(report.Native) > 0 {
		_, _ = fmt.Fprintln(out, "\n"+ui.TextBold.Render("themis-native checks")+ui.TextMuted.Render(" (no matching finding):"))
		for _, f := range report.Native {
			status := ui.LabelSuccess.Render("✓ satisfied")
			if !f.Satisfied {
				status = ui.LabelWarning.Render("○ not satisfied")
			}
			_, _ = fmt.Fprintf(out, "  %s %s — %s\n", status, f.TestID, f.Description)
			if !f.Satisfied {
				_, _ = fmt.Fprintf(out, "      run %s\n", ui.TextCommand.Render("themis apply"))
			}
		}
	}
}
