// Package fix implements checkable, revertible hardening actions. Each
// Fix maps to a Lynis test ID (or a themis-native ID for checks with no
// Lynis equivalent) and knows how to detect, apply, and revert itself.
package fix

// Fix describes one hardening action: how to detect current state, apply
// the fix, and revert it using the data Apply returned.
type Fix struct {
	Check  func() (satisfied bool, err error)
	Apply  func() (revertData []byte, err error)
	Revert func(revertData []byte) error
	// Warn, if set, is checked by `apply` before Apply runs. It reports a
	// situation this fix can't safely reason about on its own (e.g. another
	// tool already managing the same surface) so the fix is skipped and the
	// message shown instead of applied outright. A caller can still force
	// the apply through once they've reviewed it.
	Warn func() (message string, detected bool, err error)
	// SetTrust, if set, means this fix can affect trusted networks/IPs
	// (e.g. fail2ban's ignoreip allowlist) — `apply` resolves a CIDR to
	// exempt (interactively, or from --yes/--trust) and calls SetTrust with
	// it (or "" for no exemption) before Apply runs.
	SetTrust func(cidr string)
	TestID   string
	// LynisID is the raw Lynis test ID this fix addresses, when it
	// differs from TestID (e.g. one Lynis finding split across several
	// fixes). Empty means LynisID == TestID.
	LynisID     string
	Description string
}

// LynisTestID returns the raw Lynis test ID this fix addresses.
func (f Fix) LynisTestID() string { //nolint:gocritic // value receiver kept so map-index call sites (e.g. Registry[id].LynisTestID()) stay addressable
	if f.LynisID != "" {
		return f.LynisID
	}
	return f.TestID
}

// Registry maps test IDs to their Fix. Built once as plain data; callers
// (cmd/plan.go, cmd/apply.go) are the only code that invokes the funcs.
var Registry = map[string]Fix{
	"SSH-7408-ROOTLOGIN":  sshPermitRootLoginFix(),
	"SSH-7408-PASSWDAUTH": sshPasswordAuthFix(),
	"FIRE-4590":           firewallDefaultDenyFix(),
	"PKGS-7392":           autoUpdatesFix(),
	"KRNL-6000":           sysctlHardeningFix(),
	"THEMIS-FAIL2BAN":     fail2banFix(),
}
