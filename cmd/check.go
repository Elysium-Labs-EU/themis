package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
	"github.com/Elysium-Labs-EU/themis/internal/checkreport"
	"github.com/Elysium-Labs-EU/themis/internal/lynis"
	"github.com/Elysium-Labs-EU/themis/internal/native"
	"github.com/Elysium-Labs-EU/themis/internal/osquery"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

// sources lists every audit source themis runs when auditing the system
// (themis check, themis api check). quick, when true, runs lynis with its
// lighter --quick profile instead of a full audit. osquery.NewSource is
// always included: it no-ops (no findings, no error) on a host with no
// prior `themis apply` state or no osqueryi binary installed, so it's
// safe to run unconditionally.
func sources(quick bool) []audit.Source {
	return []audit.Source{lynis.NewSource(lynis.Options{Quick: quick}), native.NewSource(), osquery.NewSource("")}
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
			printCheckReport(cmd.OutOrStdout(), report, showAll)
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

// partitionFindings splits findings into those to show and those to
// de-emphasize (not actionable, unless showAll is set). Pure — no I/O.
func partitionFindings(findings []checkreport.Finding, showAll bool) (shown, deemphasized []checkreport.Finding) {
	shown = make([]checkreport.Finding, 0, len(findings))
	deemphasized = make([]checkreport.Finding, 0, len(findings))
	for i := range findings {
		if !findings[i].Actionable && !showAll {
			deemphasized = append(deemphasized, findings[i])
			continue
		}
		shown = append(shown, findings[i])
	}
	return shown, deemphasized
}

// printDeemphasized lists the findings themis can't act on directly, with a
// pointer to `themis check --all`. No-op when there are none.
func printDeemphasized(out io.Writer, deemphasized []checkreport.Finding) {
	if len(deemphasized) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "\n%s %d finding(s) themis can't act on directly (no fix, no solution hint):\n",
		ui.TextMuted.Render("i"), len(deemphasized))
	for i := range deemphasized {
		_, _ = fmt.Fprintf(out, "  %s\n", ui.TextMuted.Render(deemphasized[i].TestID+" — "+deemphasized[i].Description))
	}
	_, _ = fmt.Fprintf(out, "  run %s for full details\n", ui.TextCommand.Render("themis check --all"))
}

// printNativeChecks lists themis-native checks that matched no source
// finding, flagging any that aren't satisfied. No-op when there are none.
func printNativeChecks(out io.Writer, native []checkreport.Fix) {
	if len(native) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, "\n"+ui.TextBold.Render("themis-native checks")+ui.TextMuted.Render(" (no matching finding):"))
	for _, f := range native {
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

// printDrift lists fixes that a prior `themis apply` run confirmed
// satisfied but that osquery now reports as no longer holding. Printed
// ahead of the regular findings, and styled as an error rather than a
// warning/suggestion, since a drifted fix is a regression on something
// themis already fixed once — distinct from a finding that was never
// addressed. No-op when there is none.
func printDrift(out io.Writer, drift []checkreport.Finding) {
	if len(drift) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "%s %d fix(es) have drifted since they were last applied:\n",
		ui.LabelError.Render("!"), len(drift))
	for i := range drift {
		_, _ = fmt.Fprintf(out, "  %s %s\n", ui.TextBold.Render(drift[i].TestID), drift[i].Description)
		if drift[i].Details != "" {
			_, _ = fmt.Fprintf(out, "      %s\n", ui.TextMuted.Render(drift[i].Details))
		}
	}
	_, _ = fmt.Fprintf(out, "  run %s to re-apply\n\n", ui.TextCommand.Render("themis apply"))
}

func printCheckReport(out io.Writer, report checkreport.Report, showAll bool) {
	printDrift(out, report.Drift)

	shown, deemphasized := partitionFindings(report.Findings, showAll)

	_, _ = fmt.Fprintf(out, "%s audit reported %d finding(s)\n\n", ui.LabelInfo.Render("i"), len(report.Findings))

	for i := range shown {
		if i > 0 {
			_, _ = fmt.Fprintln(out)
		}
		printFindingBlock(out, &shown[i])
	}

	printDeemphasized(out, deemphasized)
	printNativeChecks(out, report.Native)
}
