package cmd

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/checkreport"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

func TestPartitionFindings(t *testing.T) {
	findings := []checkreport.Finding{
		{TestID: "A", Actionable: true},
		{TestID: "B", Actionable: false},
	}

	shown, deemph := partitionFindings(findings, false)
	if len(shown) != 1 || shown[0].TestID != "A" {
		t.Fatalf("shown = %+v, want just A", shown)
	}
	if len(deemph) != 1 || deemph[0].TestID != "B" {
		t.Fatalf("deemphasized = %+v, want just B", deemph)
	}

	shownAll, deemphAll := partitionFindings(findings, true)
	if len(shownAll) != 2 || len(deemphAll) != 0 {
		t.Fatalf("with showAll: shown=%d deemph=%d, want 2 and 0", len(shownAll), len(deemphAll))
	}
}

func TestPrintCheckReport(t *testing.T) {
	report := checkreport.Report{
		Findings: []checkreport.Finding{
			{
				TestID: "SSH-7408", Kind: "warning", Description: "harden ssh", Actionable: true,
				Fixes: []checkreport.Fix{{TestID: "SSH-7408-ROOTLOGIN", Satisfied: false}},
			},
			{TestID: "MISC-0001", Kind: "suggestion", Description: "minor thing", Actionable: false},
		},
		Native: []checkreport.Fix{
			{TestID: "THEMIS-FAIL2BAN", Description: "fail2ban jail", Satisfied: false},
			{TestID: "THEMIS-OK", Description: "already ok", Satisfied: true},
		},
	}

	buf := &bytes.Buffer{}
	printCheckReport(buf, report, false)
	out := buf.String()

	for _, want := range []string{
		"audit reported 2 finding(s)",
		"SSH-7408",
		"themis fix:",
		"can't act on directly", // deemphasized block header
		"MISC-0001",
		"themis-native checks",
		"THEMIS-FAIL2BAN",
		"not satisfied",
		"themis apply",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}

func TestPrintCheckReportShowAllHidesNothing(t *testing.T) {
	report := checkreport.Report{
		Findings: []checkreport.Finding{
			{TestID: "MISC-0001", Kind: "suggestion", Description: "minor", Actionable: false},
		},
	}

	buf := &bytes.Buffer{}
	printCheckReport(buf, report, true)
	if strings.Contains(buf.String(), "can't act on directly") {
		t.Errorf("with --all nothing should be de-emphasized:\n%s", buf.String())
	}
}

func TestCheckCmdRunEErrorsWithoutLynisBinary(t *testing.T) {
	if _, err := exec.LookPath("lynis"); err == nil {
		t.Skip("lynis is installed on this host; skipping the missing-binary path")
	}

	buf := &bytes.Buffer{}
	checkCmd.SetOut(buf)
	checkCmd.SetContext(context.Background())
	defer checkCmd.SetOut(nil)

	err := checkCmd.RunE(checkCmd, nil)
	if err == nil {
		t.Fatal("expected an error when the lynis binary is missing")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if uerr.Hint == "" {
		t.Error("expected a hint pointing at how to install lynis")
	}
}
