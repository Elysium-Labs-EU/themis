// Package binpath resolves external command names to absolute paths by
// checking a fixed set of trusted system directories — never $PATH.
// themis runs as root; resolving a bare command name through the
// process's inherited $PATH would let anything planted earlier in that
// (possibly attacker-influenced) search order execute as root instead of
// the real tool.
//
// Resolving themis's own top-level exec.Command call isn't enough on its
// own: some of the tools themis spawns (lynis in particular) shell out to
// further bare-name commands (dpkg, sysctl, ...) internally, and by
// default a child process inherits the parent's full environment,
// $PATH included. Environ pins that inherited $PATH to TrustedDirs too,
// so a poisoned $PATH can't reach those grandchild execs either.
package binpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TrustedDirs are the only directories themis resolves external commands
// from. This mirrors sudo's default secure_path and covers every location
// Debian's package manager installs system tools to.
var TrustedDirs = []string{
	"/usr/sbin",
	"/usr/bin",
	"/sbin",
	"/bin",
	"/usr/local/sbin",
	"/usr/local/bin",
}

// SanitizedPath is the $PATH value Environ pins on every spawned child
// process.
var SanitizedPath = strings.Join(TrustedDirs, ":")

// Resolve finds name in TrustedDirs and returns its absolute path. It
// never consults $PATH or any other environment-controlled search order.
func Resolve(name string) (string, error) {
	return resolveIn(name, TrustedDirs, fileExists)
}

// Environ returns a copy of base (typically os.Environ()) with PATH
// overridden to SanitizedPath, preserving every other variable. Spawned
// commands should set cmd.Env to this rather than leaving Env nil (which
// inherits the parent's $PATH unmodified) — a bare-name exec inside the
// child would otherwise still be free to pick up whatever an attacker
// planted earlier in the process's original $PATH. Pure — no I/O.
func Environ(base []string) []string {
	env := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if strings.HasPrefix(kv, "PATH=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "PATH="+SanitizedPath)
}

// resolveIn does the actual search, with dirs/exists parameterized so
// tests can drive it without touching the real filesystem.
func resolveIn(name string, dirs []string, exists func(string) bool) (string, error) {
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if exists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s: not found in trusted dirs %v", name, dirs)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
