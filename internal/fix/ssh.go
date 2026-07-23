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
	f := sshDisableDirectiveFixAt("SSH-7408-ROOTLOGIN", "disable SSH root login (PermitRootLogin no)", sshdConfigPath, reloadSSHD, "PermitRootLogin")
	f.LynisID = "SSH-7408"
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
		homes, err := homeDirs()
		if err != nil {
			return nil, fmt.Errorf("listing home directories: %w", err)
		}
		ok, err := authorizedKeysExist(homes)
		if err != nil {
			return nil, fmt.Errorf("checking for authorized_keys: %w", err)
		}
		if !ok {
			return nil, errors.New("refusing to disable SSH password authentication: no user has an authorized_keys file, this would lock you out")
		}
		return applyDirective()
	}
	return f
}

// authorizedKeysExist reports whether any of the given home directories
// has a non-empty .ssh/authorized_keys, i.e. whether pubkey login is
// actually possible. Guards sshPasswordAuthFix: disabling password auth
// without this would permanently lock out password-only accounts, since
// there is no fallback short of console access.
func authorizedKeysExist(homes []string) (bool, error) {
	for _, home := range homes {
		content, existed, err := ReadFileOrEmpty(filepath.Join(home, ".ssh", "authorized_keys"))
		if err != nil {
			return false, err
		}
		if existed && len(strings.TrimSpace(string(content))) > 0 {
			return true, nil
		}
	}
	return false, nil
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
	// applied renders the content Apply writes given original, the
	// pre-apply content — shared by Apply and RevertWarn so drift detection
	// compares against exactly what Apply produced.
	applied := func(original []byte) string {
		return setDirective(string(original), key, value)
	}
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
			if err := writeFile(path, []byte(applied(original)), 0o600); err != nil {
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
		RevertWarn: func(original []byte) (string, bool, error) {
			return revertDrifted(path, applied(original))
		},
	}
}
