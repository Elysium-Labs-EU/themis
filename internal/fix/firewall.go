package fix

import (
	"encoding/json"
	"fmt"
	"strings"
)

type firewallState struct {
	PrevDefaultIncoming string   `json:"prev_default_incoming"`
	AllowedPorts        []string `json:"allowed_ports"`
	WasActive           bool     `json:"was_active"`
	WasInstalled        bool     `json:"was_installed"`
}

func firewallDefaultDenyFix() Fix {
	return firewallDefaultDenyFixWith(sshdConfigPath, runCmd, runCmdOutput, packageInstalled)
}

// firewallDefaultDenyFixWith builds the FIRE-4590 fix with effect seams
// parameterized, so Check/Apply/Revert are unit-testable with fake ufw
// runners instead of touching the real firewall.
func firewallDefaultDenyFixWith(sshdPath string, run cmdRunner, outRun outputRunner, pkgInstalled pkgChecker) Fix {
	return Fix{
		TestID:      "FIRE-4590",
		Description: "enable ufw with a default-deny incoming policy while preserving SSH access",
		Check:       func() (bool, error) { return firewallCheck(sshdPath, outRun) },
		Apply:       func() ([]byte, error) { return firewallApply(sshdPath, run, outRun, pkgInstalled) },
		Revert:      func(data []byte) error { return firewallRevert(data, run) },
	}
}

// firewallCheck reports whether ufw is active with a default-deny incoming
// policy AND an allow rule for every port sshd is configured to listen on,
// reading `ufw status verbose` via the injected runner and sshd_config from
// sshdPath. Requiring the SSH allow rule (not just default-deny) matters
// because a box that is already active+deny-incoming with no SSH rule is
// exactly the locked-out state issue #18 describes — Check must not report
// that as "satisfied".
func firewallCheck(sshdPath string, outRun outputRunner) (bool, error) {
	out, err := outRun("ufw", "status", "verbose")
	if err != nil {
		return false, nil //nolint:nilerr // ufw not installed/runnable means the check is simply unsatisfied
	}
	if !strings.Contains(out, "Status: active") || parseDefaultIncoming(out) != "deny" {
		return false, nil
	}
	content, _, err := ReadFileOrEmpty(sshdPath)
	if err != nil {
		return false, err
	}
	for _, port := range sshAllowPorts(string(content)) {
		if !ufwAllowsPort(out, port) {
			return false, nil
		}
	}
	return true, nil
}

// firewallApply installs ufw if needed, captures prior state, allows the
// port(s) sshd is actually configured to listen on, sets the default-deny
// incoming policy, enables ufw, and returns the JSON revert state.
//
// The allow rule(s) must be added before the default-deny policy takes
// effect: without them, enabling ufw with default-deny-incoming and no
// exception immediately cuts off any operator connected over SSH, with no
// way back in short of console/IPMI access (issue #18). Only ports that
// weren't already allowed are recorded in AllowedPorts, so Revert removes
// exactly what this Apply added rather than a rule that predates themis.
func firewallApply(sshdPath string, run cmdRunner, outRun outputRunner, pkgInstalled pkgChecker) ([]byte, error) {
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

	content, _, readErr := ReadFileOrEmpty(sshdPath)
	if readErr != nil {
		return nil, readErr
	}
	for _, port := range sshAllowPorts(string(content)) {
		if ufwAllowsPort(out, port) {
			continue // already allowed before we touched anything; not ours to revert
		}
		if err := run("ufw", "allow", port+"/tcp"); err != nil {
			return nil, err
		}
		state.AllowedPorts = append(state.AllowedPorts, port)
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

// firewallRevert removes ufw if we installed it (which also drops any rule
// it holds, including the SSH allow rule Apply added), otherwise deletes the
// SSH allow rule(s) Apply added, restores the prior default-incoming policy,
// and disables ufw if it wasn't active before.
func firewallRevert(data []byte, run cmdRunner) error {
	var state firewallState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshaling firewall revert state: %w", err)
	}
	if !state.WasInstalled {
		return run("apt-get", "remove", "-y", "ufw")
	}
	for _, port := range state.AllowedPorts {
		if err := run("ufw", "delete", "allow", port+"/tcp"); err != nil {
			return err
		}
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

// sshAllowPorts returns the port(s) ufw must allow before a default-deny
// incoming policy is enabled, derived from sshd's own configured "Port"
// directive(s) in sshdConfig content. Unlike PermitRootLogin/
// PasswordAuthentication (single-value directives where DirectiveValue
// resolves to the last match), sshd treats every uncommented "Port" line as
// an additional listener, so all of them are collected rather than just the
// last. Defaults to "22" — sshd's own built-in default — when no Port
// directive is set, which is what keeps a stock Debian/Ubuntu box reachable
// after FIRE-4590 applies (issue #18). Pure — no I/O.
func sshAllowPorts(sshdConfig string) []string {
	var ports []string
	for _, line := range strings.Split(sshdConfig, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "Port") {
			ports = append(ports, fields[1])
		}
	}
	if len(ports) == 0 {
		return []string{"22"}
	}
	return ports
}

// ufwAllowsPort reports whether `ufw status verbose` output already shows an
// ALLOW rule for port (as "<port>/tcp"). Port 22 also matches ufw's built-in
// "OpenSSH" app profile, since operators commonly `ufw allow OpenSSH` rather
// than the raw port number. Pure — no I/O.
func ufwAllowsPort(statusOutput, port string) bool {
	for _, line := range strings.Split(statusOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(line, "ALLOW") {
			continue
		}
		if fields[0] == port+"/tcp" {
			return true
		}
		if port == "22" && fields[0] == "OpenSSH" {
			return true
		}
	}
	return false
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
