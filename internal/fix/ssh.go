package fix

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sshdConfigPath = "/etc/ssh/sshd_config"

func sshPermitRootLoginFix() Fix {
	return sshPermitRootLoginFixWith(sshdConfigPath, reloadSSHD, systemHomeDirectories)
}

// sshPermitRootLoginFixWith builds the SSH-7408-ROOTLOGIN fix with path,
// reload, and homeDirs parameterized for testability (mirrors
// sshPasswordAuthFixWith's rationale below). Its Apply is guarded by
// sshLockoutGuardOK the same way sshPasswordAuthFixWith's is: disabling
// root login with no usable key left anywhere would lock out a
// root-key-only host, since PermitRootLogin no blocks root SSH access
// entirely, key or password.
func sshPermitRootLoginFixWith(path string, reload func() error, homeDirs func() ([]string, error)) Fix {
	f := sshDisableDirectiveFixAt("SSH-7408-ROOTLOGIN", "disable SSH root login (PermitRootLogin no)", path, reload, "PermitRootLogin")
	f.LynisID = "SSH-7408"
	applyDirective := f.Apply
	f.Apply = func() ([]byte, error) {
		ok, err := sshLockoutGuardOK(path, homeDirs, "PasswordAuthentication")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("refusing to disable SSH root login: no non-root account has a usable authorized_keys entry, and password authentication is (or will be) disabled — this would lock you out")
		}
		return applyDirective()
	}
	return f
}

func sshPasswordAuthFix() Fix {
	return sshPasswordAuthFixWith(sshdConfigPath, reloadSSHD, systemHomeDirectories)
}

// sshPasswordAuthFixWith builds the SSH-7408-PASSWDAUTH fix with path,
// reload, and homeDirs parameterized for testability (mirrors
// sshDisableDirectiveFixAt's rationale below).
func sshPasswordAuthFixWith(path string, reload func() error, homeDirs func() ([]string, error)) Fix {
	f := sshDisableDirectiveFixAt("SSH-7408-PASSWDAUTH", "disable SSH password authentication (PasswordAuthentication no)", path, reload, "PasswordAuthentication")
	f.LynisID = "SSH-7408"
	applyDirective := f.Apply
	f.Apply = func() ([]byte, error) {
		ok, err := sshLockoutGuardOK(path, homeDirs, "PermitRootLogin")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("refusing to disable SSH password authentication: no non-root account has a usable authorized_keys entry, and root login is (or will be) disabled — this would lock you out")
		}
		return applyDirective()
	}
	return f
}

// sshLockoutGuardOK reads the sshd config at path and resolves homeDirs,
// then applies sshAuthorizedKeysGuard against otherDirective — the
// SSH-7408 directive the calling fix does not itself manage. Both
// sshPermitRootLoginFixWith and sshPasswordAuthFixWith call this from
// their Apply, so each accounts for the other fix having already run (or
// being about to, later in the very same apply pass): a themis run
// applies every unsatisfied fix in one sequential pass, writing each
// change to disk immediately, so whichever of the two runs second always
// observes the first one's already-persisted directive (see F-015).
func sshLockoutGuardOK(path string, homeDirs func() ([]string, error), otherDirective string) (bool, error) {
	content, _, err := ReadFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	homes, err := homeDirs()
	if err != nil {
		return false, fmt.Errorf("listing home directories: %w", err)
	}
	return sshAuthorizedKeysGuard(string(content), homes, otherDirective)
}

// sshAuthorizedKeysGuard reports whether it is safe to disable the SSH-
// 7408 directive being guarded, given sshd_config's current content and
// the system's home directories. If otherDirective (the sibling SSH-7408
// directive this fix does not manage) is already "no", root's own key
// stops being a usable fallback — PermitRootLogin no blocks root SSH
// entirely, and PasswordAuthentication no removes the password fallback —
// so only a non-root account's key (for pubkey login plus sudo/su to
// root) keeps the host reachable, and the guard requires one. Otherwise
// the sibling directive is still permissive, so any account's key
// (including root's) is an adequate fallback, mirroring the simpler
// single-fix guard.
func sshAuthorizedKeysGuard(content string, homes []string, otherDirective string) (bool, error) {
	if DirectiveValue(content, otherDirective) == "no" {
		return nonRootAuthorizedKeysExist(homes)
	}
	return authorizedKeysExist(homes)
}

// authorizedKeyTypes lists the SSH public-key type identifiers OpenSSH
// recognizes as the first field of an authorized_keys line. A line whose
// first field isn't one of these is a comment, is blank, or has an
// options prefix (e.g. command="...", no-pty, restrict) ahead of the key
// type — either way it is not a plain, unrestricted key a locked-out
// operator could use to get an interactive shell.
var authorizedKeyTypes = map[string]bool{
	"ssh-rsa":                                     true,
	"ssh-dss":                                     true,
	"ssh-ed25519":                                 true,
	"ecdsa-sha2-nistp256":                         true,
	"ecdsa-sha2-nistp384":                         true,
	"ecdsa-sha2-nistp521":                         true,
	"sk-ecdsa-sha2-nistp256@openssh.com":          true,
	"sk-ssh-ed25519@openssh.com":                  true,
	"ssh-rsa-cert-v01@openssh.com":                true,
	"ssh-dss-cert-v01@openssh.com":                true,
	"ssh-ed25519-cert-v01@openssh.com":            true,
	"ecdsa-sha2-nistp256-cert-v01@openssh.com":    true,
	"ecdsa-sha2-nistp384-cert-v01@openssh.com":    true,
	"ecdsa-sha2-nistp521-cert-v01@openssh.com":    true,
	"sk-ecdsa-sha2-nistp256-cert-v01@openssh.com": true,
	"sk-ssh-ed25519-cert-v01@openssh.com":         true,
}

// hasUsableAuthorizedKey reports whether content (an authorized_keys
// file's contents) has at least one real, unrestricted public-key line.
// Blank lines and comments don't count, and neither does a line carrying
// an options prefix (e.g. a forced "command=" or "no-pty") ahead of the
// key type, since that key can't be relied on as an interactive login
// fallback. Pure — no I/O.
func hasUsableAuthorizedKey(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && authorizedKeyTypes[fields[0]] {
			return true
		}
	}
	return false
}

// authorizedKeysExist reports whether any of the given home directories
// has a usable .ssh/authorized_keys — i.e. at least one real,
// unrestricted key, not just a comment-only file or a restricted
// ("command=..."-style) key — meaning pubkey login is actually possible.
// Guards sshPasswordAuthFix and sshPermitRootLoginFix: disabling either
// without this would risk permanently locking out accounts that have no
// usable fallback, since there is no recovery short of console access.
func authorizedKeysExist(homes []string) (bool, error) {
	for _, home := range homes {
		content, existed, err := ReadFileOrEmpty(filepath.Join(home, ".ssh", "authorized_keys"))
		if err != nil {
			return false, err
		}
		if existed && hasUsableAuthorizedKey(string(content)) {
			return true, nil
		}
	}
	return false, nil
}

// nonRootAuthorizedKeysExist is authorizedKeysExist restricted to homes
// other than root's own ("/root", the path every homeDirs implementation
// in this package guarantees is present — see homeDirectoriesFrom). Once
// both PermitRootLogin and PasswordAuthentication are "no", root's own
// key stops being a usable login path at all (see sshAuthorizedKeysGuard),
// so this is what determines whether a non-root pubkey login — with
// sudo/su to root — is still possible.
func nonRootAuthorizedKeysExist(homes []string) (bool, error) {
	nonRoot := make([]string, 0, len(homes))
	for _, h := range homes {
		if h == "/root" {
			continue
		}
		nonRoot = append(nonRoot, h)
	}
	return authorizedKeysExist(nonRoot)
}

// systemHomeDirectories reads /etc/passwd for the real system.
func systemHomeDirectories() ([]string, error) {
	return homeDirectoriesFrom("/etc/passwd")
}

// homeDirectoriesFrom returns every home directory listed in passwdPath,
// plus /root explicitly (some minimal images omit a root passwd entry).
func homeDirectoriesFrom(passwdPath string) ([]string, error) {
	f, err := os.Open(passwdPath) //nolint:gosec // passwdPath is a fixed constant at the real call site; parameterized for tests
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", passwdPath, err)
	}
	defer func() { _ = f.Close() }()

	homes := []string{"/root"}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) >= 6 && fields[5] != "" {
			homes = append(homes, fields[5])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", passwdPath, err)
	}
	return homes, nil
}

func reloadSSHD() error {
	return runCmd("systemctl", "reload", "ssh")
}

// sshDisableDirectiveFixAt builds a Fix that sets key to "no" in the
// sshd config at path, calling reload after every mutation. path and
// reload are parameterized (rather than hardcoded) so the
// Check/Apply/Revert logic is unit-testable against a temp file with a
// no-op reload.
//
// Check/Apply only ever look at (and edit) the top-level/global scope of
// the config — see directive.go's Match-block handling — so a
// Match-scoped override of key is never misreported as "satisfied" nor
// silently rewritten. Warn additionally surfaces when such an override
// exists, so an operator relying on it is told a fix touching the same
// directive's global default is about to run, mirroring the fail2ban
// fix's Warn for "can't fully reason about this on its own" situations.
func sshDisableDirectiveFixAt(testID, description, path string, reload func() error, key string) Fix {
	const value = "no"
	return Fix{
		TestID:      testID,
		Description: description,
		Warn: func() (string, bool, error) {
			content, _, err := ReadFileOrEmpty(path)
			if err != nil {
				return "", false, err
			}
			if !directiveInMatchBlock(string(content), key) {
				return "", false, nil
			}
			return fmt.Sprintf(
				"%s is also set inside a Match block in %s — themis only manages the global default and will not touch the Match-scoped override, but review it before proceeding to confirm it's still what you intend",
				key, path,
			), true, nil
		},
		Check: func() (bool, error) {
			content, _, err := ReadFileOrEmpty(path)
			if err != nil {
				return false, err
			}
			return DirectiveValue(string(content), key) == value, nil
		},
		Apply: func() ([]byte, error) {
			original, _, err := ReadFileOrEmpty(path)
			if err != nil {
				return nil, err
			}
			updated := setDirective(string(original), key, value)
			if err := writeFile(path, []byte(updated), 0o600); err != nil {
				return nil, err
			}
			if err := reload(); err != nil {
				return original, err
			}
			return original, nil
		},
		Revert: func(original []byte) error {
			if err := writeFile(path, original, 0o600); err != nil {
				return err
			}
			return reload()
		},
	}
}
