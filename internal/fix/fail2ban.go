package fix

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const fail2banJailLocalPath = "/etc/fail2ban/jail.local"

// banactionMultiport scopes a ban to the jail's own port (e.g. sshd's port
// 22) instead of the whole offending IP. Without this pin, a distro's
// default banaction — or an iptables-allports override elsewhere in the
// fail2ban config — can collaterally block unrelated services (e.g. a VPN)
// that happen to share the banned IP.
const banactionMultiport = "iptables-multiport"

type fail2banState struct {
	PrevConfig    []byte `json:"prev_config"`
	WasInstalled  bool   `json:"was_installed"`
	ConfigExisted bool   `json:"config_existed"`
}

// fail2banFix has no Lynis equivalent — it demonstrates a themis-native
// check registered outside the Lynis-wrap path.
func fail2banFix() Fix {
	return fail2banFixWith(fail2banJailLocalPath, runCmd, packageInstalled)
}

// fail2banFixWith builds the THEMIS-FAIL2BAN fix with the jail.local path
// and effect seams parameterized, so the Check/Apply/Revert logic is
// unit-testable against a temp file with fake runners (mirrors
// sshDisableDirectiveFixAt's rationale).
func fail2banFixWith(path string, run cmdRunner, pkgInstalled pkgChecker) Fix {
	return Fix{
		TestID:      "THEMIS-FAIL2BAN",
		Description: "install and enable fail2ban with an sshd jail",
		Warn:        fail2banWarn,
		Check:       func() (bool, error) { return fail2banCheck(path, run) },
		Apply:       func() ([]byte, error) { return fail2banApply(path, run, pkgInstalled) },
		Revert:      func(data []byte) error { return fail2banRevert(data, path, run) },
		RevertWarn:  func(data []byte) (string, bool, error) { return fail2banRevertWarn(data, path) },
	}
}

// fail2banCheck reports whether fail2ban is active with an enabled, port-
// scoped [sshd] jail in the config at path. Effects (systemctl, file read)
// are at the edges via the injected runner.
func fail2banCheck(path string, run cmdRunner) (bool, error) {
	if err := run("systemctl", "is-active", "--quiet", "fail2ban"); err != nil {
		return false, nil //nolint:nilerr // service not active means the check is simply unsatisfied
	}
	content, existed, err := ReadFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	return existed && SSHDJailEnabled(string(content)) && SSHDBanactionScoped(string(content)), nil
}

// fail2banApply installs fail2ban if needed, patches the [sshd] jail into
// the config at path, enables the service, and returns the JSON revert state.
func fail2banApply(path string, run cmdRunner, pkgInstalled pkgChecker) ([]byte, error) {
	wasInstalled := pkgInstalled("fail2ban")
	if !wasInstalled {
		if err := run("apt-get", "install", "-y", "fail2ban"); err != nil {
			return nil, err
		}
	}
	original, existed, err := ReadFileOrEmpty(path)
	if err != nil {
		return nil, err
	}
	updated := ensureSSHDJail(string(original))
	if writeErr := writeFile(path, []byte(updated), 0o644); writeErr != nil {
		return nil, writeErr
	}
	if enableErr := run("systemctl", "enable", "--now", "fail2ban"); enableErr != nil {
		return nil, enableErr
	}
	state := fail2banState{WasInstalled: wasInstalled, PrevConfig: original, ConfigExisted: existed}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling fail2ban revert state: %w", err)
	}
	return data, nil
}

// fail2banRevert restores the config at path (or removes it if it didn't
// exist) and undoes the install/enable using the JSON revert state.
func fail2banRevert(data []byte, path string, run cmdRunner) error {
	var state fail2banState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshaling fail2ban revert state: %w", err)
	}
	if state.ConfigExisted {
		if err := writeFile(path, state.PrevConfig, 0o644); err != nil {
			return err
		}
	} else if err := removeFile(path); err != nil {
		return err
	}
	if !state.WasInstalled {
		_ = run("systemctl", "disable", "--now", "fail2ban")
		return run("apt-get", "remove", "-y", "fail2ban")
	}
	return run("systemctl", "restart", "fail2ban")
}

// fail2banRevertWarn reports whether path currently differs from the
// [sshd] jail content Apply wrote, i.e. it was hand-edited since apply and
// Revert would discard that edit unless warned first.
func fail2banRevertWarn(data []byte, path string) (string, bool, error) {
	var state fail2banState
	if err := json.Unmarshal(data, &state); err != nil {
		return "", false, fmt.Errorf("unmarshaling fail2ban revert state: %w", err)
	}
	return revertDrifted(path, ensureSSHDJail(string(state.PrevConfig)))
}

// fail2banWarn flags hosts where pinning banaction ourselves may not be the
// right call. A WireGuard endpoint often routes many distinct users behind
// one shared tunnel IP, so a port-scoped ban can still be too coarse for
// that traffic. CrowdSec manages its own bans through the same
// iptables/nftables surface fail2ban does, so two ban managers touching
// overlapping rules can conflict in ways themis has no visibility into.
// Rather than silently rewrite banaction either way, surface what was found
// and let the operator decide.
func fail2banWarn() (string, bool, error) {
	msg, detected := fail2banWarnMessage(wireguardConfigured(), crowdsecActive())
	return msg, detected, nil
}

// fail2banWarnMessage builds the advisory message from what was detected.
// Pure — no I/O — so the wording/joining logic is testable without faking
// systemctl or the filesystem.
func fail2banWarnMessage(hasWireguard, hasCrowdsec bool) (string, bool) {
	var found []string
	if hasWireguard {
		found = append(found, "a WireGuard config")
	}
	if hasCrowdsec {
		found = append(found, "CrowdSec")
	}
	if len(found) == 0 {
		return "", false
	}
	msg := fmt.Sprintf(
		"detected %s on this host — pinning fail2ban's [sshd] banaction to %s changes how it bans IPs; review whether that fits your setup, then rerun apply with --force once you have",
		strings.Join(found, " and "), banactionMultiport,
	)
	return msg, true
}

// wireguardConfigured reports whether any WireGuard interface config is
// present, regardless of whether the interface is currently up.
func wireguardConfigured() bool {
	entries, err := os.ReadDir("/etc/wireguard")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".conf") {
			return true
		}
	}
	return false
}

// crowdsecActive reports whether CrowdSec or its firewall bouncer is
// running — either can already be managing bans on this host.
func crowdsecActive() bool {
	return runCmd("systemctl", "is-active", "--quiet", "crowdsec") == nil ||
		runCmd("systemctl", "is-active", "--quiet", "crowdsec-firewall-bouncer") == nil
}

// SSHDJailEnabled reports whether jail.local has "enabled = true" inside
// its [sshd] section. Pure — no I/O. Exported for internal/native, which
// checks the same config for the THEMIS-FAIL2BAN finding.
func SSHDJailEnabled(content string) bool {
	return sectionHasKeyValue(content, "sshd", "enabled", "true")
}

// SSHDBanactionScoped reports whether jail.local pins the [sshd] section's
// banaction to iptables-multiport, so a ban only blocks the jail's port
// rather than the whole IP. Pure — no I/O. Exported for internal/native,
// which checks the same config for the THEMIS-FAIL2BAN finding.
func SSHDBanactionScoped(content string) bool {
	return sectionHasKeyValue(content, "sshd", "banaction", banactionMultiport)
}

// sectionHasKeyValue reports whether "key = value" appears verbatim inside
// the named INI section. Pure — no I/O.
func sectionHasKeyValue(content, section, key, value string) bool {
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inSection = trimmed == "["+section+"]"
			continue
		}
		if inSection && trimmed == key+" = "+value {
			return true
		}
	}
	return false
}

// ensureSSHDJail makes sure jail.local has a [sshd] section that is enabled
// and pins banaction to iptables-multiport, creating or patching the
// section as needed. backend = systemd is used instead of the default
// logpath because Debian 12+ has no /var/log/auth.log without rsyslog
// installed — sshd logs to the journal only, and fail2ban refuses to start
// if its configured logpath doesn't exist. Pure — no I/O.
func ensureSSHDJail(content string) string {
	if !hasSection(content, "sshd") {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + "\n[sshd]\nenabled = true\nbackend = systemd\nbanaction = " + banactionMultiport + "\n"
	}
	content = setSectionKey(content, "sshd", "enabled", "true")
	return setSectionKey(content, "sshd", "banaction", banactionMultiport)
}

// hasSection reports whether content contains a "[section]" header.
func hasSection(content, section string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "["+section+"]" {
			return true
		}
	}
	return false
}

// setSectionKey ensures "key = value" appears exactly once within the named
// section, updating an existing "key = ..." line in place or appending one
// at the end of the section if absent. Assumes the section already exists.
func setSectionKey(content, section, key, value string) string {
	lines := strings.Split(content, "\n")
	target := key + " = " + value
	inSection := false
	sectionEnd := -1
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if inSection && sectionEnd == -1 {
				sectionEnd = i
			}
			inSection = trimmed == "["+section+"]"
			continue
		}
		if inSection {
			sectionEnd = i + 1
			if strings.HasPrefix(trimmed, key+" =") {
				lines[i] = target
				found = true
			}
		}
	}
	if found {
		return strings.Join(lines, "\n")
	}
	if sectionEnd == -1 {
		sectionEnd = len(lines)
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:sectionEnd]...)
	out = append(out, target)
	out = append(out, lines[sectionEnd:]...)
	return strings.Join(out, "\n")
}
