package osquery

import (
	"sort"
	"strings"
)

// DriftCheck maps one internal/fix registry TestID to an osquery query
// that independently re-verifies it still holds. Pure — Satisfied only
// interprets rows already fetched by Query.
type DriftCheck struct {
	Satisfied   func(rows []Row) bool
	TestID      string
	Description string
	Query       string
}

// sysctlDesired mirrors internal/fix's sysctlHardeningSettings: the
// kernel network parameters KRNL-6000 sets via a sysctl drop-in.
var sysctlDesired = map[string]string{
	"net.ipv4.conf.all.accept_source_route":     "0",
	"net.ipv4.conf.default.accept_source_route": "0",
	"net.ipv4.conf.all.send_redirects":          "0",
	"net.ipv4.conf.default.send_redirects":      "0",
	"net.ipv4.conf.all.accept_redirects":        "0",
	"net.ipv4.conf.default.accept_redirects":    "0",
	"net.ipv4.tcp_syncookies":                   "1",
	"net.ipv4.icmp_echo_ignore_broadcasts":      "1",
}

// sysctlQuery builds the system_controls query for every key in
// sysctlDesired, in a stable (sorted) order so the query string itself
// is deterministic and diffable.
func sysctlQuery() string {
	names := make([]string, 0, len(sysctlDesired))
	for name := range sysctlDesired {
		names = append(names, name)
	}
	sort.Strings(names)

	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "'" + name + "'"
	}
	return "SELECT name, current_value FROM system_controls WHERE name IN (" + strings.Join(quoted, ", ") + ")"
}

// sysctlsHold reports whether every sysctlDesired key is present in rows
// with its desired value. Pure — no I/O.
func sysctlsHold(rows []Row) bool {
	got := make(map[string]string, len(rows))
	for _, r := range rows {
		got[r["name"]] = r["current_value"]
	}
	for name, want := range sysctlDesired {
		if got[name] != want {
			return false
		}
	}
	return true
}

// sshdConfigDenies builds a Satisfied func for a single sshd_config key
// whose query already filters to that key: satisfied when any returned
// row's value matches want. Pure — no I/O.
func sshdConfigDenies(want string) func([]Row) bool {
	return func(rows []Row) bool {
		for _, r := range rows {
			if strings.EqualFold(r["value"], want) {
				return true
			}
		}
		return false
	}
}

// fail2banActive reports whether systemd_units shows fail2ban.service in
// the "active" state. Pure — no I/O.
func fail2banActive(rows []Row) bool {
	return len(rows) > 0 && rows[0]["active_state"] == "active"
}

// Checks is every fix.Registry TestID this package can independently
// re-verify via osquery, and how.
//
// FIRE-4590 (firewall default-deny) and PKGS-7392 (unattended upgrades)
// are deliberately not covered here: FIRE-4590 has no single osquery
// table that works the same way across ufw, nftables, and raw iptables
// setups, and PKGS-7392 depends on both a package and a config file
// together, which osquery's package tables alone can't resolve. Both
// stay covered by the existing lynis/native sources instead.
var Checks = []DriftCheck{
	{
		TestID:      "SSH-7408-ROOTLOGIN",
		Description: "SSH root login was re-enabled since it was last hardened",
		Query:       "SELECT value FROM sshd_config WHERE key = 'permitrootlogin'",
		Satisfied:   sshdConfigDenies("no"),
	},
	{
		TestID:      "SSH-7408-PASSWDAUTH",
		Description: "SSH password authentication was re-enabled since it was last hardened",
		Query:       "SELECT value FROM sshd_config WHERE key = 'passwordauthentication'",
		Satisfied:   sshdConfigDenies("no"),
	},
	{
		TestID:      "KRNL-6000",
		Description: "kernel network hardening sysctls reverted since they were last applied",
		Query:       sysctlQuery(),
		Satisfied:   sysctlsHold,
	},
	{
		TestID:      "THEMIS-FAIL2BAN",
		Description: "fail2ban stopped running since it was last enabled",
		Query:       "SELECT active_state FROM systemd_units WHERE id = 'fail2ban.service'",
		Satisfied:   fail2banActive,
	},
}
