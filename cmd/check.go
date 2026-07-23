package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
	"github.com/Elysium-Labs-EU/themis/internal/checkreport"
	"github.com/Elysium-Labs-EU/themis/internal/lynis"
	"github.com/Elysium-Labs-EU/themis/internal/native"
	"github.com/Elysium-Labs-EU/themis/internal/openscap"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

// sources lists every audit source themis runs when auditing the system
// (themis check, themis api check). quick, when true, runs lynis with its
// lighter --quick profile instead of a full audit. scapContent, when
// non-empty, adds an OpenSCAP source evaluating that SCAP/XCCDF
// datastream (optionally scoped to scapProfile); empty leaves OpenSCAP
// out entirely, since — unlike lynis — it's not a themis dependency and
// most hosts won't have SCAP content installed.
func sources(quick bool, scapContent, scapProfile string) []audit.Source {
	srcs := []audit.Source{lynis.NewSource(lynis.Options{Quick: quick}), native.NewSource()}
	if scapContent != "" {
		srcs = append(srcs, openscap.NewSource(openscap.Options{ContentPath: scapContent, Profile: scapProfile}))
	}
	return srcs
}

func newCheckCmd() *cobra.Command {
	var showAll bool
	var quick bool
	var scapContent string
	var scapProfile string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run an audit and list actionable findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var findings []audit.Finding
			err := ui.WithSpinner("Running audit...", func() error {
				var err error
				findings, err = audit.Run(cmd.Context(), sources(quick, scapContent, scapProfile))
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
	cmd.Flags().StringVar(&scapContent, "scap-content", "", "path to a SCAP/XCCDF datastream (e.g. oscap-ssg content); also runs OpenSCAP when set")
	cmd.Flags().StringVar(&scapProfile, "scap-profile", "", "XCCDF profile ID to evaluate (default: the datastream's own default profile)")
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
	for _, f := range findings {
		if !f.Actionable && !showAll {
			deemphasized = append(deemphasized, f)
			continue
		}
		shown = append(shown, f)
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
	for _, f := range deemphasized {
		_, _ = fmt.Fprintf(out, "  %s\n", ui.TextMuted.Render(f.TestID+" — "+f.Description))
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

func printCheckReport(out io.Writer, report checkreport.Report, showAll bool) {
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
