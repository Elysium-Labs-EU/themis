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
	"github.com/Elysium-Labs-EU/themis/internal/osquery"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
)

// sources lists every audit source themis runs when auditing the system
// (themis check, themis api check). osquery.NewSource is always included:
// it no-ops (no findings, no error) on a host with no prior `themis apply`
// state or no osqueryi binary installed, so it's safe to run
// unconditionally. scapContent, when non-empty, adds an OpenSCAP source
// evaluating that SCAP/XCCDF datastream (optionally scoped to scapProfile);
// empty leaves OpenSCAP out entirely, since — unlike lynis — it's not a
// themis dependency and most hosts won't have SCAP content installed.
func sources(lynisOpts lynis.Options, scapContent, scapProfile string) []audit.Source {
	srcs := []audit.Source{lynis.NewSource(lynisOpts), native.NewSource(), osquery.NewSource("")}
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
	var skipUnchanged bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run an audit and list actionable findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var findings []audit.Finding
			err := ui.WithSpinner("Running audit...", func() error {
				var err error
				lynisOpts := lynis.Options{Quick: quick, SkipIfUnchanged: skipUnchanged}
				findings, err = audit.Run(cmd.Context(), sources(lynisOpts, scapContent, scapProfile))
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
	cmd.Flags().BoolVar(&skipUnchanged, "skip-unchanged", false, "skip the lynis scan and reuse the last report if nothing lynis cares about (config files, package list) has changed since")
	return cmd
}

var checkCmd = newCheckCmd()

func fixSummary(fixes []checkreport.Fix) string {
	parts := make([]string, 0, len(fixes))
	for _, f := range fixes {
		icon := ui.FixIcon("○ apply", ui.StatusPending)
		if f.Satisfied {
			icon = ui.FixIcon("✓ fixed", ui.StatusSatisfied)
		}
		parts = append(parts, f.TestID+" "+icon)
	}
	return strings.Join(parts, ", ")
}

// friendlySourceName renders an audit.Source.Name() for section headers
// and inline tags. Falls back to the raw name for sources with no
// special-cased label.
func friendlySourceName(source string) string {
	switch source {
	case "lynis":
		return "Lynis"
	case "themis":
		return "themis-native"
	case "openscap":
		return "OpenSCAP"
	default:
		return source
	}
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
	if len(f.Sources) > 1 {
		extra := make([]string, len(f.Sources)-1)
		for i, s := range f.Sources[1:] {
			extra[i] = friendlySourceName(s)
		}
		_, _ = fmt.Fprintf(out, "  %s %s\n", ui.TextMuted.Render("also reported by:"), strings.Join(extra, ", "))
	}
}

// sourceGroup is every finding whose primary (first-reporting) source is
// Source, in original order.
type sourceGroup struct {
	Source   string
	Findings []checkreport.Finding
}

// groupBySource splits findings by primary source — f.Sources[0], the
// source that reported it first — preserving each finding's original
// order within its group and ordering groups by first appearance. This
// is what makes "who asserted this" visible without reading source code:
// a Lynis finding and a themis-native finding never share a section.
// Pure — no I/O.
func groupBySource(findings []checkreport.Finding) []sourceGroup {
	order := make([]string, 0)
	bySource := make(map[string][]checkreport.Finding)
	for i := range findings {
		src := "unknown"
		if len(findings[i].Sources) > 0 {
			src = findings[i].Sources[0]
		}
		if _, ok := bySource[src]; !ok {
			order = append(order, src)
		}
		bySource[src] = append(bySource[src], findings[i])
	}
	groups := make([]sourceGroup, 0, len(order))
	for _, src := range order {
		groups = append(groups, sourceGroup{Source: src, Findings: bySource[src]})
	}
	return groups
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
		source := "unknown"
		if len(deemphasized[i].Sources) > 0 {
			source = friendlySourceName(deemphasized[i].Sources[0])
		}
		_, _ = fmt.Fprintf(out, "  %s\n", ui.TextMuted.Render(deemphasized[i].TestID+" ("+source+") — "+deemphasized[i].Description))
	}
	_, _ = fmt.Fprintf(out, "  run %s for full details\n", ui.TextCommand.Render("themis check --all"))
}

// printUnmatchedFixes lists themis fixes whose tracked test ID was not
// reported by any audit source this run, flagging any that aren't
// satisfied. Not specific to themis-native fixes — a Lynis-tracked fix
// lands here just as readily when Lynis's own scan found nothing wrong
// with it. No-op when there are none.
func printUnmatchedFixes(out io.Writer, unmatched []checkreport.Fix) {
	if len(unmatched) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, "\n"+ui.TextBold.Render("unmatched themis fixes")+ui.TextMuted.Render(" (no finding from any source this run):"))
	for _, f := range unmatched {
		status := ui.FixIcon("✓ satisfied", ui.StatusSatisfied)
		if !f.Satisfied {
			status = ui.FixIcon("○ not satisfied", ui.StatusPending)
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

	groups := groupBySource(shown)
	for gi, g := range groups {
		if gi > 0 {
			_, _ = fmt.Fprintln(out)
		}
		_, _ = fmt.Fprintln(out, ui.TextBold.Render(friendlySourceName(g.Source)+" findings")+ui.TextMuted.Render(":"))
		for i := range g.Findings {
			if i > 0 {
				_, _ = fmt.Fprintln(out)
			}
			printFindingBlock(out, &g.Findings[i])
		}
	}

	printDeemphasized(out, deemphasized)
	printUnmatchedFixes(out, report.Unmatched)
}
