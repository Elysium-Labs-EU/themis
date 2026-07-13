package fix

const sshdConfigPath = "/etc/ssh/sshd_config"

func sshPermitRootLoginFix() Fix {
	f := sshDisableDirectiveFixAt("SSH-7408-ROOTLOGIN", "disable SSH root login (PermitRootLogin no)", sshdConfigPath, reloadSSHD, "PermitRootLogin")
	f.LynisID = "SSH-7408"
	return f
}

func sshPasswordAuthFix() Fix {
	f := sshDisableDirectiveFixAt("SSH-7408-PASSWDAUTH", "disable SSH password authentication (PasswordAuthentication no)", sshdConfigPath, reloadSSHD, "PasswordAuthentication")
	f.LynisID = "SSH-7408"
	return f
}

func reloadSSHD() error {
	return runCmd("systemctl", "reload", "ssh")
}

// sshDisableDirectiveFixAt builds a Fix that sets key to "no" in the
// sshd config at path, calling reload after every mutation. path and
// reload are parameterized (rather than hardcoded) so the
// Check/Apply/Revert logic is unit-testable against a temp file with a
// no-op reload.
func sshDisableDirectiveFixAt(testID, description, path string, reload func() error, key string) Fix {
	const value = "no"
	return Fix{
		TestID:      testID,
		Description: description,
		Check: func() (bool, error) {
			content, _, err := readFileOrEmpty(path)
			if err != nil {
				return false, err
			}
			return directiveValue(string(content), key) == value, nil
		},
		Apply: func() ([]byte, error) {
			original, _, err := readFileOrEmpty(path)
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
