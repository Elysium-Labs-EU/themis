package native

import (
	"context"

	"codeberg.org/Elysium_Labs/themis/internal/audit"
	"codeberg.org/Elysium_Labs/themis/internal/fix"
)

const fail2banJailLocalPath = "/etc/fail2ban/jail.local"

// fail2banFinding reports THEMIS-FAIL2BAN when fail2ban isn't active,
// has no enabled sshd jail, or hasn't pinned that jail's banaction to
// scope bans to the port instead of the whole IP. Its TestID matches the
// existing THEMIS-FAIL2BAN fix (internal/fix/fail2ban.go), which checks
// the same three conditions, so checkreport.Build pairs them
// automatically. Returns nil when the check is satisfied — findings
// represent problems, not passing checks.
func fail2banFinding(ctx context.Context) (*audit.Finding, error) {
	active := runCmd(ctx, "systemctl", "is-active", "--quiet", "fail2ban") == nil

	var jailEnabled, banactionScoped bool
	if active {
		content, existed, err := fix.ReadFileOrEmpty(fail2banJailLocalPath)
		if err != nil {
			return nil, err
		}
		jailEnabled = existed && fix.SSHDJailEnabled(string(content))
		banactionScoped = existed && fix.SSHDBanactionScoped(string(content))
	}

	return fail2banDecision(active, jailEnabled, banactionScoped), nil
}

// fail2banDecision is the pure check behind fail2banFinding: given the
// service's active state and its jail.local settings, decide whether a
// finding is warranted. Pure — no I/O.
func fail2banDecision(active, jailEnabled, banactionScoped bool) *audit.Finding {
	switch {
	case !active:
		return &audit.Finding{
			TestID:      "THEMIS-FAIL2BAN",
			Description: "fail2ban is not installed or not active",
			Details:     "-",
			Solution:    "-",
			Kind:        "suggestion",
			Source:      "themis",
		}
	case !jailEnabled:
		return &audit.Finding{
			TestID:      "THEMIS-FAIL2BAN",
			Description: "fail2ban is active but has no enabled sshd jail",
			Details:     fail2banJailLocalPath,
			Solution:    "-",
			Kind:        "suggestion",
			Source:      "themis",
		}
	case !banactionScoped:
		return &audit.Finding{
			TestID:      "THEMIS-FAIL2BAN",
			Description: "fail2ban's sshd jail does not scope bans to the port (banaction)",
			Details:     fail2banJailLocalPath,
			Solution:    "-",
			Kind:        "suggestion",
			Source:      "themis",
		}
	default:
		return nil
	}
}
