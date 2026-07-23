package fix

const sysctlDropInPath = "/etc/sysctl.d/60-themis-hardening.conf"

var sysctlHardeningSettings = []string{
	"net.ipv4.conf.all.accept_source_route = 0",
	"net.ipv4.conf.default.accept_source_route = 0",
	"net.ipv4.conf.all.send_redirects = 0",
	"net.ipv4.conf.default.send_redirects = 0",
	"net.ipv4.conf.all.accept_redirects = 0",
	"net.ipv4.conf.default.accept_redirects = 0",
	"net.ipv4.tcp_syncookies = 1",
	"net.ipv4.icmp_echo_ignore_broadcasts = 1",
}

func reloadSysctl() error {
	return runCmd("sysctl", "--system")
}

func sysctlHardeningFix() Fix {
	return sysctlFixAt(sysctlDropInPath, reloadSysctl)
}

// sysctlFixAt builds the sysctl drop-in Fix against path, calling reload
// after every mutation. path and reload are parameterized so the logic
// is unit-testable against a temp file with a no-op reload.
func sysctlFixAt(path string, reload func() error) Fix {
	desired := buildSysctlDropIn()
	return Fix{
		TestID:      "KRNL-6000",
		Description: "harden kernel network parameters via a sysctl drop-in file",
		Check: func() (bool, error) {
			content, existed, err := ReadFileOrEmpty(path)
			if err != nil {
				return false, err
			}
			return existed && string(content) == desired, nil
		},
		Apply: func() ([]byte, error) {
			original, existed, err := ReadFileOrEmpty(path)
			if err != nil {
				return nil, err
			}
			if err := writeFile(path, []byte(desired), 0o644); err != nil {
				return nil, err
			}
			if err := reload(); err != nil {
				return nil, err
			}
			if !existed {
				return nil, nil // nil sentinel: file did not exist before Apply
			}
			return original, nil
		},
		Revert: func(original []byte) error {
			if original == nil {
				if err := removeFile(path); err != nil {
					return err
				}
			} else if err := writeFile(path, original, 0o644); err != nil {
				return err
			}
			return reload()
		},
		RevertWarn: func([]byte) (string, bool, error) {
			return revertDrifted(path, desired)
		},
	}
}

// buildSysctlDropIn renders the desired drop-in file content. Pure — no I/O.
func buildSysctlDropIn() string {
	content := "# managed by themis\n"
	for _, line := range sysctlHardeningSettings {
		content += line + "\n"
	}
	return content
}
