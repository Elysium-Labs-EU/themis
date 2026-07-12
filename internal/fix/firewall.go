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
	return Fix{
		TestID:      "FIRE-4590",
		Description: "enable ufw with a default-deny incoming policy",
		Check: func() (bool, error) {
			out, err := runCmdOutput("ufw", "status", "verbose")
			if err != nil {
				return false, nil //nolint:nilerr // ufw not installed/runnable means the check is simply unsatisfied
			}
			return strings.Contains(out, "Status: active") && parseDefaultIncoming(out) == "deny", nil
		},
		Apply: func() ([]byte, error) {
			wasInstalled := packageInstalled("ufw")
			if !wasInstalled {
				if err := runCmd("apt-get", "install", "-y", "ufw"); err != nil {
					return nil, err
				}
			}
			out, _ := runCmdOutput("ufw", "status", "verbose")
			state := firewallState{
				WasActive:           strings.Contains(out, "Status: active"),
				PrevDefaultIncoming: parseDefaultIncoming(out),
				WasInstalled:        wasInstalled,
			}
			if err := runCmd("ufw", "default", "deny", "incoming"); err != nil {
				return nil, err
			}
			if err := runCmd("ufw", "--force", "enable"); err != nil {
				return nil, err
			}
			data, err := json.Marshal(state)
			if err != nil {
				return nil, fmt.Errorf("marshaling firewall revert state: %w", err)
			}
			return data, nil
		},
		Revert: func(data []byte) error {
			var state firewallState
			if err := json.Unmarshal(data, &state); err != nil {
				return fmt.Errorf("unmarshaling firewall revert state: %w", err)
			}
			if !state.WasInstalled {
				return runCmd("apt-get", "remove", "-y", "ufw")
			}
			if state.PrevDefaultIncoming != "" {
				if err := runCmd("ufw", "default", state.PrevDefaultIncoming, "incoming"); err != nil {
					return err
				}
			}
			if !state.WasActive {
				return runCmd("ufw", "--force", "disable")
			}
			return nil
		},
	}
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
