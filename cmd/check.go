package cmd

import (
	"fmt"
	"strings"

	"codeberg.org/Elysium_Labs/themis/internal/checkreport"
	"codeberg.org/Elysium_Labs/themis/internal/lynis"
	"codeberg.org/Elysium_Labs/themis/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run a Lynis audit and list actionable findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			findings, err := lynis.Audit(cmd.Context())
			if err != nil {
				return fmt.Errorf("running lynis audit: %w", err)
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

	cmd.Flags().BoolVar(&showAll, "all", false, "also show findings with no themis fix and no Lynis solution hint")
	return cmd
}

var checkCmd = newCheckCmd()

func fixSummary(fixes []checkreport.Fix) string {
	if len(fixes) == 0 {
		return ui.TableMutedStyle.Render("-")
	}
	parts := make([]string, 0, len(fixes))
	for _, f := range fixes {
		icon := ui.LabelWarning.Render("○ apply")
		if f.Satisfied {
			icon = ui.LabelSuccess.Render("✓ fixed")
		}
		parts = append(parts, f.TestID+" "+icon)
	}
	return strings.Join(parts, "\n")
}

func printCheckReport(cmd *cobra.Command, report checkreport.Report, showAll bool) {
	out := cmd.OutOrStdout()

	shown := make([]checkreport.Finding, 0, len(report.Findings))
	hidden := 0
	for _, f := range report.Findings {
		if !f.Actionable && !showAll {
			hidden++
			continue
		}
		shown = append(shown, f)
	}

	_, _ = fmt.Fprintf(out, "%s Lynis reported %d finding(s)\n\n", ui.LabelInfo.Render("i"), len(report.Findings))

	rows := make([][]string, 0, len(shown))
	for _, f := range shown {
		kind := ui.TextMuted.Render(f.Kind)
		if f.Kind == "warning" {
			kind = ui.LabelWarning.Render(f.Kind)
		}
		solution := ui.TableMutedStyle.Render("-")
		if f.Solution != "" && f.Solution != "-" {
			solution = f.Solution
		}
		rows = append(rows, []string{f.TestID, kind, f.Description, solution, fixSummary(f.Fixes)})
	}

	if len(rows) > 0 {
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(ui.TableBorderColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return ui.TableHeaderStyle
				}
				if row%2 == 0 {
					return ui.TableEvenRowStyle
				}
				return ui.TableOddRowStyle
			}).
			Headers("test id", "kind", "description", "lynis solution", "themis fix").
			Rows(rows...)
		_, _ = fmt.Fprintln(out, t)
	}

	if hidden > 0 {
		_, _ = fmt.Fprintf(out, "\n%s %d finding(s) with no themis fix and no Lynis solution hidden — run %s to see them\n",
			ui.TextMuted.Render("i"), hidden, ui.TextCommand.Render("themis check --all"))
	}

	if len(report.Native) > 0 {
		_, _ = fmt.Fprintln(out, "\n"+ui.TextBold.Render("themis-native checks")+ui.TextMuted.Render(" (no Lynis finding to match):"))
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
