package fix

import (
	"encoding/json"
	"fmt"
	"strings"
)

type firewallState struct {
	PrevDefaultIncoming string `json:"prev_default_incoming"`
	WasActive           bool   `json:"was_active"`
	WasInstalled        bool   `json:"was_installed"`
}

func firewallDefaultDenyFix() Fix {
	return firewallDefaultDenyFixWith(runCmd, runCmdOutput, packageInstalled)
}

// firewallDefaultDenyFixWith builds the FIRE-4590 fix with effect seams
// parameterized, so Check/Apply/Revert are unit-testable with fake ufw
// runners instead of touching the real firewall.
func firewallDefaultDenyFixWith(run cmdRunner, outRun outputRunner, pkgInstalled pkgChecker) Fix {
	return Fix{
		TestID:      "FIRE-4590",
		Description: "enable ufw with a default-deny incoming policy",
		Check:       func() (bool, error) { return firewallCheck(outRun) },
		Apply:       func() ([]byte, error) { return firewallApply(run, outRun, pkgInstalled) },
		Revert:      func(data []byte) error { return firewallRevert(data, run) },
	}
}

// firewallCheck reports whether ufw is active with a default-deny incoming
// policy, reading `ufw status verbose` via the injected runner.
func firewallCheck(outRun outputRunner) (bool, error) {
	out, err := outRun("ufw", "status", "verbose")
	if err != nil {
		return false, nil //nolint:nilerr // ufw not installed/runnable means the check is simply unsatisfied
	}
	return strings.Contains(out, "Status: active") && parseDefaultIncoming(out) == "deny", nil
}

// firewallApply installs ufw if needed, captures prior state, sets the
// default-deny incoming policy, enables ufw, and returns the JSON revert state.
func firewallApply(run cmdRunner, outRun outputRunner, pkgInstalled pkgChecker) ([]byte, error) {
	wasInstalled := pkgInstalled("ufw")
	if !wasInstalled {
		if err := run("apt-get", "install", "-y", "ufw"); err != nil {
			return nil, err
		}
	}
	out, _ := outRun("ufw", "status", "verbose")
	state := firewallState{
		WasActive:           strings.Contains(out, "Status: active"),
		PrevDefaultIncoming: parseDefaultIncoming(out),
		WasInstalled:        wasInstalled,
	}
	if err := run("ufw", "default", "deny", "incoming"); err != nil {
		return nil, err
	}
	if err := run("ufw", "--force", "enable"); err != nil {
		return nil, err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling firewall revert state: %w", err)
	}
	return data, nil
}

// firewallRevert removes ufw if we installed it, otherwise restores the prior
// default-incoming policy and disables ufw if it wasn't active before.
func firewallRevert(data []byte, run cmdRunner) error {
	var state firewallState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshaling firewall revert state: %w", err)
	}
	if !state.WasInstalled {
		return run("apt-get", "remove", "-y", "ufw")
	}
	if state.PrevDefaultIncoming != "" {
		if err := run("ufw", "default", state.PrevDefaultIncoming, "incoming"); err != nil {
			return err
		}
	}
	if !state.WasActive {
		return run("ufw", "--force", "disable")
	}
	return nil
}

// parseDefaultIncoming extracts the incoming default policy ("deny",
// "allow", "reject", or "" if not found) from `ufw status verbose`
// output. Pure — no I/O.
func parseDefaultIncoming(statusOutput string) string {
	for _, line := range strings.Split(statusOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Default:") {
			continue
		}
		for _, policy := range []string{"deny", "allow", "reject"} {
			if strings.Contains(trimmed, policy+" (incoming)") {
				return policy
			}
		}
	}
	return ""
}
