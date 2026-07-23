package osquery

import (
	"strings"
	"testing"
)

func findCheck(t *testing.T, testID string) DriftCheck {
	t.Helper()
	for _, c := range Checks {
		if c.TestID == testID {
			return c
		}
	}
	t.Fatalf("no DriftCheck registered for %s", testID)
	return DriftCheck{}
}

func TestSSHRootLoginSatisfiedWhenDenied(t *testing.T) {
	c := findCheck(t, "SSH-7408-ROOTLOGIN")
	if !c.Satisfied([]Row{{"value": "no"}}) {
		t.Error("expected PermitRootLogin=no to satisfy the check")
	}
	if !c.Satisfied([]Row{{"value": "No"}}) {
		t.Error("expected a case-insensitive match to satisfy the check")
	}
}

func TestSSHRootLoginDriftedWhenReenabled(t *testing.T) {
	c := findCheck(t, "SSH-7408-ROOTLOGIN")
	if c.Satisfied([]Row{{"value": "yes"}}) {
		t.Error("expected PermitRootLogin=yes to be reported as drifted")
	}
	if c.Satisfied(nil) {
		t.Error("expected no sshd_config row at all to be reported as drifted")
	}
}

func TestSSHPasswordAuthSatisfiedWhenDenied(t *testing.T) {
	c := findCheck(t, "SSH-7408-PASSWDAUTH")
	if !c.Satisfied([]Row{{"value": "no"}}) {
		t.Error("expected PasswordAuthentication=no to satisfy the check")
	}
}

func TestSSHPasswordAuthDriftedWhenReenabled(t *testing.T) {
	c := findCheck(t, "SSH-7408-PASSWDAUTH")
	if c.Satisfied([]Row{{"value": "yes"}}) {
		t.Error("expected PasswordAuthentication=yes to be reported as drifted")
	}
}

func TestSysctlsHoldWhenAllDesiredValuesMatch(t *testing.T) {
	rows := make([]Row, 0, len(sysctlDesired))
	for name, value := range sysctlDesired {
		rows = append(rows, Row{"name": name, "current_value": value})
	}
	if !sysctlsHold(rows) {
		t.Error("expected all sysctls at their desired value to satisfy the check")
	}
}

func TestSysctlsDriftedWhenOneValueReverts(t *testing.T) {
	rows := make([]Row, 0, len(sysctlDesired))
	first := true
	for name, value := range sysctlDesired {
		if first {
			value = "9" // simulate a reverted value
			first = false
		}
		rows = append(rows, Row{"name": name, "current_value": value})
	}
	if sysctlsHold(rows) {
		t.Error("expected one reverted sysctl to be reported as drifted")
	}
}

func TestSysctlsDriftedWhenKeyMissingEntirely(t *testing.T) {
	if sysctlsHold(nil) {
		t.Error("expected no rows at all to be reported as drifted")
	}
}

func TestFail2banActiveWhenSystemdReportsActive(t *testing.T) {
	if !fail2banActive([]Row{{"active_state": "active"}}) {
		t.Error("expected active_state=active to satisfy the check")
	}
}

func TestFail2banDriftedWhenSystemdReportsInactiveOrMissing(t *testing.T) {
	if fail2banActive([]Row{{"active_state": "inactive"}}) {
		t.Error("expected active_state=inactive to be reported as drifted")
	}
	if fail2banActive(nil) {
		t.Error("expected no systemd_units row at all to be reported as drifted")
	}
}

func TestSysctlQueryIncludesEveryDesiredKey(t *testing.T) {
	q := sysctlQuery()
	for name := range sysctlDesired {
		if !strings.Contains(q, name) {
			t.Errorf("query %q missing sysctl key %q", q, name)
		}
	}
}
