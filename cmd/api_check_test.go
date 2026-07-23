package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApiCheckResultSerializesDriftSeparatelyFromFindings(t *testing.T) {
	result := apiCheckResult{
		Findings: []apiFinding{{TestID: "SSH-7408", Kind: "warning"}},
		Drift: []apiDrift{
			{TestID: "THEMIS-FAIL2BAN", Description: "fail2ban stopped running", Details: "applied 2026-01-01T00:00:00Z, no longer satisfied"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		`"drift":[{"test_id":"THEMIS-FAIL2BAN"`,
		`"details":"applied 2026-01-01T00:00:00Z, no longer satisfied"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}

	var round apiCheckResult
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("round-tripping: %v", err)
	}
	if len(round.Drift) != 1 || round.Drift[0].TestID != "THEMIS-FAIL2BAN" {
		t.Fatalf("round-tripped Drift = %+v", round.Drift)
	}
	if len(round.Findings) != 1 || round.Findings[0].TestID != "SSH-7408" {
		t.Fatalf("round-tripped Findings = %+v, expected drift not to have leaked in", round.Findings)
	}
}
