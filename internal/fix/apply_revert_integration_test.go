//go:build integration

package fix

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// Integration tests here drive each host-mutating Fix's real Apply then
// Revert against the live system and verify the effect with INDEPENDENT
// tools (systemctl, dpkg, grep, ufw) — never the fix's own Check(). They
// require:
//   - Linux
//   - root
//   - apt/dpkg (Debian/Ubuntu); fail2ban + firewall additionally need
//     systemd as PID 1 to enable/start services
//   - working apt network to install the package under test
//
// Run via: make test-integration
//   or on OrbStack: orb run -m debian -u root bash -lc \
//     "export PATH=/usr/local/go/bin:\$PATH; cd <repo> && go test ./internal/fix/... -tags integration -v"
//
// Missing prerequisites t.Skip rather than fail, so the suite is safe on a
// laptop or a container without systemd. Every case reverts through
// t.Cleanup even if an assertion fails partway, so a failed run still
// restores the host.

func requireLinuxRoot(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux")
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
}

func requireApt(t *testing.T) {
	t.Helper()
	requireLinuxRoot(t)
	for _, bin := range []string{"apt-get", "dpkg"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("requires %s (Debian/Ubuntu host)", bin)
		}
	}
}

func requireSystemd(t *testing.T) {
	t.Helper()
	// /run/systemd/system exists iff systemd is the init system and running.
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		t.Skip("requires systemd as PID 1 (service enable/start)")
	}
}

// requireMutationOptIn gates the host-mutating apply/revert tests behind an
// explicit opt-in. They apt-get install packages, enable services, and touch
// ufw — fine on a throwaway OrbStack VM (make test-integration-orb sets the
// env), but they must NOT run on the shared/networkless CI runner, where they
// would fail rather than skip. Unset env => skip.
func requireMutationOptIn(t *testing.T) {
	t.Helper()
	if os.Getenv("THEMIS_INTEGRATION_MUTATE") != "1" {
		t.Skip("host-mutating; set THEMIS_INTEGRATION_MUTATE=1 (make test-integration-orb) to run")
	}
}

// mustRun runs a command and returns its combined output, failing the test
// on a non-nil error. For verification steps that MUST succeed.
func mustRun(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput() //nolint:gosec // fixed literals in a test
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// tryRun runs a command and returns (output, ok). ok is false on a non-zero
// exit — used where a non-zero exit is a legitimate observation (e.g.
// `systemctl is-active` on a stopped service).
func tryRun(name string, args ...string) (string, bool) {
	out, err := exec.Command(name, args...).CombinedOutput() //nolint:gosec // fixed literals in a test
	return string(out), err == nil
}

// applyWithGuaranteedRevert runs the fix's Apply and registers a Cleanup
// that reverts exactly once. It returns a revert func the test body can call
// early (to assert restoration) without the Cleanup double-reverting.
func applyWithGuaranteedRevert(t *testing.T, f Fix) (revert func()) {
	t.Helper()
	revertData, err := f.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	reverted := false
	revert = func() {
		if reverted {
			return
		}
		reverted = true
		if rerr := f.Revert(revertData); rerr != nil {
			t.Errorf("Revert: %v", rerr)
		}
	}
	t.Cleanup(revert)
	return revert
}

// snapshotFile reads path, reporting existence and raw bytes, so a revert
// can be asserted byte-identical to the pre-apply state.
func snapshotFile(t *testing.T, path string) (content []byte, existed bool) {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // fixed config path
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		t.Fatalf("reading %s: %v", path, err)
	}
	return b, true
}

func assertRestored(t *testing.T, path string, before []byte, existedBefore bool) {
	t.Helper()
	after, existedAfter := snapshotFile(t, path)
	if existedAfter != existedBefore {
		t.Errorf("%s existence after revert = %v, want %v", path, existedAfter, existedBefore)
		return
	}
	if !bytes.Equal(after, before) {
		t.Errorf("%s not byte-identical after revert\n--- before (%d bytes) ---\n%s\n--- after (%d bytes) ---\n%s",
			path, len(before), before, len(after), after)
	}
}

func TestFail2banApplyRevertIntegration(t *testing.T) {
	requireMutationOptIn(t)
	requireApt(t)
	requireSystemd(t)

	before, existedBefore := snapshotFile(t, fail2banJailLocalPath)

	revert := applyWithGuaranteedRevert(t, fail2banFix())

	// Independent verification the fix took effect.
	if out, ok := tryRun("systemctl", "is-active", "fail2ban"); !ok {
		t.Errorf("systemctl is-active fail2ban did not report active:\n%s", out)
	}
	jail := mustRun(t, "cat", fail2banJailLocalPath)
	if !strings.Contains(jail, "[sshd]") {
		t.Errorf("jail.local missing [sshd] section:\n%s", jail)
	}
	if !strings.Contains(jail, "enabled = true") {
		t.Errorf("jail.local [sshd] not enabled:\n%s", jail)
	}
	if !strings.Contains(jail, "banaction = "+banactionMultiport) {
		t.Errorf("jail.local banaction not pinned to %s:\n%s", banactionMultiport, jail)
	}

	revert()

	assertRestored(t, fail2banJailLocalPath, before, existedBefore)
}

func TestAutoUpdatesApplyRevertIntegration(t *testing.T) {
	requireMutationOptIn(t)
	requireApt(t)

	before, existedBefore := snapshotFile(t, autoUpgradesConfigPath)

	revert := applyWithGuaranteedRevert(t, autoUpdatesFix())

	// Independent verification: package installed and the directive set.
	if _, ok := tryRun("dpkg", "-s", "unattended-upgrades"); !ok {
		t.Error("dpkg -s unattended-upgrades reports it is not installed after apply")
	}
	conf := mustRun(t, "cat", autoUpgradesConfigPath)
	if !strings.Contains(conf, `APT::Periodic::Unattended-Upgrade "1";`) {
		t.Errorf("%s missing the Unattended-Upgrade directive:\n%s", autoUpgradesConfigPath, conf)
	}

	revert()

	assertRestored(t, autoUpgradesConfigPath, before, existedBefore)
}

func TestFirewallDefaultDenyApplyRevertIntegration(t *testing.T) {
	requireMutationOptIn(t)
	requireApt(t)
	requireSystemd(t)

	// Capture prior ufw state so we can assert it is restored. `ufw status`
	// fails entirely when ufw isn't installed — a legitimate prior state.
	prevStatus, prevInstalled := tryRun("ufw", "status", "verbose")
	prevActive := strings.Contains(prevStatus, "Status: active")
	prevDefault := parseDefaultIncoming(prevStatus)

	// Determine which port(s) this box's sshd is actually configured to
	// listen on, the same way firewallApply does, so the assertion below
	// checks the port that matters on this host rather than assuming 22.
	sshdConfig, _, err := ReadFileOrEmpty(sshdConfigPath)
	if err != nil {
		t.Fatalf("reading %s: %v", sshdConfigPath, err)
	}
	wantPorts := sshAllowPorts(string(sshdConfig))

	revert := applyWithGuaranteedRevert(t, firewallDefaultDenyFix())

	// Independent verification: ufw active with default-deny incoming AND
	// an allow rule for sshd's configured port(s) — issue #18 was that
	// FIRE-4590 enabled default-deny-incoming with zero SSH allow rule,
	// severing remote access to the box being hardened.
	status := mustRun(t, "ufw", "status", "verbose")
	if !strings.Contains(status, "Status: active") {
		t.Errorf("ufw not active after apply:\n%s", status)
	}
	if got := parseDefaultIncoming(status); got != "deny" {
		t.Errorf("default incoming = %q after apply, want deny:\n%s", got, status)
	}
	for _, port := range wantPorts {
		if !ufwAllowsPort(status, port) {
			t.Errorf("no ufw ALLOW rule for SSH port %s after apply — this would lock out a remote admin:\n%s", port, status)
		}
	}

	revert()

	// Verify the SSH allow rule this fix added is gone after revert too —
	// not just the default policy — so revert doesn't leave stale rules
	// behind (only checked when ufw was already installed; otherwise the
	// package removal below already wipes every rule).
	if prevInstalled {
		afterRevertStatus := mustRun(t, "ufw", "status", "verbose")
		for _, port := range wantPorts {
			if ufwAllowsPort(afterRevertStatus, port) && !ufwAllowsPort(prevStatus, port) {
				t.Errorf("ufw ALLOW rule for port %s added by apply still present after revert:\n%s", port, afterRevertStatus)
			}
		}
	}

	// Verify prior state restored.
	if !prevInstalled {
		// ufw was absent before; revert removes it, so the command should be
		// gone (or at least not report an active firewall we introduced).
		if out, ok := tryRun("ufw", "status", "verbose"); ok && strings.Contains(out, "Status: active") {
			t.Errorf("ufw still active after revert though it was absent before:\n%s", out)
		}
		return
	}
	after := mustRun(t, "ufw", "status", "verbose")
	if gotActive := strings.Contains(after, "Status: active"); gotActive != prevActive {
		t.Errorf("ufw active after revert = %v, want prior %v:\n%s", gotActive, prevActive, after)
	}
	if got := parseDefaultIncoming(after); prevDefault != "" && got != prevDefault {
		t.Errorf("default incoming after revert = %q, want prior %q:\n%s", got, prevDefault, after)
	}
}
