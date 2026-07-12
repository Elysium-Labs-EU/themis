package fix

import (
	"encoding/json"
	"fmt"
	"strings"
)

const fail2banJailLocalPath = "/etc/fail2ban/jail.local"

type fail2banState struct {
	PrevConfig    []byte `json:"prev_config"`
	WasInstalled  bool   `json:"was_installed"`
	ConfigExisted bool   `json:"config_existed"`
}

// fail2banFix has no Lynis equivalent — it demonstrates a themis-native
// check registered outside the Lynis-wrap path.
func fail2banFix() Fix {
	return Fix{
		TestID:      "THEMIS-FAIL2BAN",
		Description: "install and enable fail2ban with an sshd jail",
		Check: func() (bool, error) {
			if err := runCmd("systemctl", "is-active", "--quiet", "fail2ban"); err != nil {
				return false, nil //nolint:nilerr // service not active means the check is simply unsatisfied
			}
			content, existed, err := readFileOrEmpty(fail2banJailLocalPath)
			if err != nil {
				return false, err
			}
			return existed && sshdJailEnabled(string(content)), nil
		},
		Apply: func() ([]byte, error) {
			wasInstalled := packageInstalled("fail2ban")
			if !wasInstalled {
				if err := runCmd("apt-get", "install", "-y", "fail2ban"); err != nil {
					return nil, err
				}
			}
			original, existed, err := readFileOrEmpty(fail2banJailLocalPath)
			if err != nil {
				return nil, err
			}
			updated := ensureSSHDJail(string(original))
			if writeErr := writeFile(fail2banJailLocalPath, []byte(updated), 0o644); writeErr != nil {
				return nil, writeErr
			}
			if enableErr := runCmd("systemctl", "enable", "--now", "fail2ban"); enableErr != nil {
				return nil, enableErr
			}
			state := fail2banState{WasInstalled: wasInstalled, PrevConfig: original, ConfigExisted: existed}
			data, err := json.Marshal(state)
			if err != nil {
				return nil, fmt.Errorf("marshaling fail2ban revert state: %w", err)
			}
			return data, nil
		},
		Revert: func(data []byte) error {
			var state fail2banState
			if err := json.Unmarshal(data, &state); err != nil {
				return fmt.Errorf("unmarshaling fail2ban revert state: %w", err)
			}
			if state.ConfigExisted {
				if err := writeFile(fail2banJailLocalPath, state.PrevConfig, 0o644); err != nil {
					return err
				}
			} else if err := removeFile(fail2banJailLocalPath); err != nil {
				return err
			}
			if !state.WasInstalled {
				_ = runCmd("systemctl", "disable", "--now", "fail2ban")
				return runCmd("apt-get", "remove", "-y", "fail2ban")
			}
			return runCmd("systemctl", "restart", "fail2ban")
		},
	}
}

// sshdJailEnabled reports whether jail.local has "enabled = true" inside
// its [sshd] section. Pure — no I/O.
func sshdJailEnabled(content string) bool {
	inSSHD := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inSSHD = trimmed == "[sshd]"
			continue
		}
		if inSSHD && trimmed == "enabled = true" {
			return true
		}
	}
	return false
}

// ensureSSHDJail appends a "[sshd]" section enabled against the systemd
// journal if the jail isn't already enabled. backend = systemd is used
// instead of the default logpath because Debian 12+ has no
// /var/log/auth.log without rsyslog installed — sshd logs to the
// journal only, and fail2ban refuses to start if its configured logpath
// doesn't exist. Pure — no I/O.
func ensureSSHDJail(content string) string {
	if sshdJailEnabled(content) {
		return content
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n[sshd]\nenabled = true\nbackend = systemd\n"
}
