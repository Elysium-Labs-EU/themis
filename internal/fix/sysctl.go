package fix

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// sysctlKV is one live kernel parameter's prior value, captured at Apply
// time so Revert can restore it explicitly rather than relying on
// sysctl --system's reload-from-files semantics, which has no way to
// "unset" a value a since-removed file used to set. A slice (not a map)
// keeps restore order matching sysctlHardeningSettings, so Revert's
// sysctl -w calls and any test assertions on them are deterministic.
type sysctlKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// sysctlState is the JSON-encoded revert data returned by Apply: the
// drop-in file's prior content (and whether it existed at all) plus the
// live value of every key Apply is about to touch.
type sysctlState struct {
	FileContent []byte     `json:"file_content"`
	PrevValues  []sysctlKV `json:"prev_values"`
	FileExisted bool       `json:"file_existed"`
}

func reloadSysctl() error {
	return runCmd("sysctl", "--system")
}

func sysctlHardeningFix() Fix {
	return sysctlFixAt(sysctlDropInPath, runCmdOutput, runCmd, reloadSysctl)
}

// sysctlFixAt builds the sysctl drop-in Fix against path, reading/writing
// live kernel values via outRun/run and calling reload after every
// mutation. path, outRun, run and reload are parameterized so the logic is
// unit-testable against a temp file with fake runners instead of the real
// sysctl.
func sysctlFixAt(path string, outRun outputRunner, run cmdRunner, reload func() error) Fix {
	desired := buildSysctlDropIn()
	desiredKV := sysctlDesiredKV()
	keys := sysctlKeys()
	return Fix{
		TestID:      "KRNL-6000",
		Description: "harden kernel network parameters via a sysctl drop-in file",
		Check: func() (bool, error) {
			content, existed, err := ReadFileOrEmpty(path)
			if err != nil {
				return false, err
			}
			if !existed || string(content) != desired {
				return false, nil
			}
			for _, kv := range desiredKV {
				got, outErr := outRun("sysctl", "-n", kv.Key)
				if outErr != nil {
					return false, nil //nolint:nilerr // unreadable live value means the check is simply unsatisfied
				}
				if strings.TrimSpace(got) != kv.Value {
					return false, nil
				}
			}
			return true, nil
		},
		Apply: func() ([]byte, error) {
			content, existed, readErr := ReadFileOrEmpty(path)
			if readErr != nil {
				return nil, readErr
			}
			prevValues := make([]sysctlKV, 0, len(keys))
			for _, key := range keys {
				val, outErr := outRun("sysctl", "-n", key)
				if outErr != nil {
					return nil, outErr
				}
				prevValues = append(prevValues, sysctlKV{Key: key, Value: strings.TrimSpace(val)})
			}
			if writeErr := writeFile(path, []byte(desired), 0o644); writeErr != nil {
				return nil, writeErr
			}
			if reloadErr := reload(); reloadErr != nil {
				return nil, reloadErr
			}
			data, err := json.Marshal(sysctlState{FileContent: content, PrevValues: prevValues, FileExisted: existed})
			if err != nil {
				return nil, fmt.Errorf("marshaling sysctl revert state: %w", err)
			}
			return data, nil
		},
		RevertWarn: func([]byte) (string, bool, error) {
			return driftWarning(path, desired)
		},
		Revert: func(data []byte) error {
			var state sysctlState
			if err := json.Unmarshal(data, &state); err != nil {
				return fmt.Errorf("unmarshaling sysctl revert state: %w", err)
			}
			if !state.FileExisted {
				if err := removeFile(path); err != nil {
					return err
				}
			} else if err := writeFile(path, state.FileContent, 0o644); err != nil {
				return err
			}
			for _, kv := range state.PrevValues {
				if err := run("sysctl", "-w", kv.Key+"="+kv.Value); err != nil {
					return err
				}
			}
			return reload()
		},
	}
}

// buildSysctlDropIn renders the desired drop-in file content. Pure — no I/O.
func buildSysctlDropIn() string {
	var b strings.Builder
	b.WriteString("# managed by themis\n")
	for _, line := range sysctlHardeningSettings {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// sysctlDesiredKV parses sysctlHardeningSettings into key/value pairs, the
// live values Check compares against so drift at the kernel level (not just
// the drop-in file) is caught. Pure — no I/O.
func sysctlDesiredKV() []sysctlKV {
	kvs := make([]sysctlKV, 0, len(sysctlHardeningSettings))
	for _, line := range sysctlHardeningSettings {
		if key, val, ok := strings.Cut(line, " = "); ok {
			kvs = append(kvs, sysctlKV{Key: key, Value: val})
		}
	}
	return kvs
}

// sysctlKeys returns the bare parameter names (e.g. "net.ipv4.tcp_syncookies")
// this fix touches. Pure — no I/O.
func sysctlKeys() []string {
	kvs := sysctlDesiredKV()
	keys := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		keys = append(keys, kv.Key)
	}
	return keys
}
