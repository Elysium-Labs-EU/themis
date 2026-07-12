// Package fix implements checkable, revertible hardening actions. Each
// Fix maps to a Lynis test ID (or a themis-native ID for checks with no
// Lynis equivalent) and knows how to detect, apply, and revert itself.
package fix

// Fix describes one hardening action: how to detect current state, apply
// the fix, and revert it using the data Apply returned.
type Fix struct {
	Check       func() (satisfied bool, err error)
	Apply       func() (revertData []byte, err error)
	Revert      func(revertData []byte) error
	TestID      string
	Description string
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
