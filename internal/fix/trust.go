package fix

import (
	"net"
	"os"
	"strings"
)

// CurrentConnectionCIDR reports the host CIDR (a /32 for IPv4, /128 for
// IPv6) of the client side of the current SSH connection, derived from the
// SSH_CONNECTION environment variable sshd sets for every session. Lets an
// interactive `apply` prompt offer "trust the connection I'm on right now"
// for a fix's SetTrust hook without the operator needing to already know
// their own IP. Returns ok=false when there's no SSH session to read (e.g.
// running at the console).
func CurrentConnectionCIDR() (cidr string, ok bool) {
	return parseConnectionCIDR(os.Getenv("SSH_CONNECTION"))
}

// parseConnectionCIDR is CurrentConnectionCIDR's pure core: SSH_CONNECTION
// is "<client ip> <client port> <server ip> <server port>"; only the client
// ip is relevant here. Pure — no I/O.
func parseConnectionCIDR(sshConnection string) (string, bool) {
	fields := strings.Fields(sshConnection)
	if len(fields) == 0 {
		return "", false
	}
	ip := net.ParseIP(fields[0])
	if ip == nil {
		return "", false
	}
	if ip.To4() != nil {
		return fields[0] + "/32", true
	}
	return fields[0] + "/128", true
}
