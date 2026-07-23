package fix

import "strings"

// setDirective ensures a "Key Value" style config (one directive per
// line, '#' comments) has key set to value within the top-level/global
// scope only: comments out other uncommented global occurrences of key
// and appends the desired line if it isn't already the last effective
// global setting. Lines at or after the first top-level `Match` line are
// left completely untouched. Pure — no I/O.
//
// The Match-block boundary matters for OpenSSH's sshd_config: directives
// inside a `Match` block only apply to the connections that match its
// condition, not globally. Rewriting or commenting a directive inside
// (or after) a Match block would silently break or hide an operator's
// deliberate per-user/per-network override, with no indication a "fix"
// just edited inside a conditional scope.
func setDirective(content, key, value string) string {
	lines := strings.Split(content, "\n")
	// A trailing newline produces one empty trailing element; drop it so
	// appending the desired directive doesn't leave a blank line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	globalEnd := len(lines)
	if idx := firstMatchLineIndex(lines); idx >= 0 {
		globalEnd = idx
	}

	out := make([]string, 0, len(lines)+1)
	found := false
	for i, line := range lines {
		if i >= globalEnd {
			out = append(out, line)
			continue
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], key) {
			if fields[1] == value {
				found = true
				out = append(out, line)
				continue
			}
			out = append(out, "#"+line)
			continue
		}
		out = append(out, line)
	}
	if !found {
		// Insert immediately before the Match block (or at the end, if
		// there is none) so the new directive stays in the global scope
		// rather than becoming Match-block scoped by accident.
		withInsert := make([]string, 0, len(out)+1)
		withInsert = append(withInsert, out[:globalEnd]...)
		withInsert = append(withInsert, key+" "+value)
		withInsert = append(withInsert, out[globalEnd:]...)
		out = withInsert
	}
	return strings.Join(out, "\n")
}

// DirectiveValue returns the first effective (uncommented) value for key
// within the top-level/global scope, or "" if it is never set there.
// This mirrors OpenSSH's actual sshd_config parsing: sshd uses the first
// occurrence of a keyword and ignores every later duplicate, so a
// repeated global directive does not override its predecessor. Reporting
// the last occurrence instead would produce a false "satisfied" verdict
// whenever an earlier, still-effective line is the insecure one — the
// real running sshd obeys the first line, not the last.
//
// Lines at or after the first top-level `Match` line are ignored: a
// Match block only redefines a directive for the connections it
// matches, so a value set there does not reflect the general-case
// posture (e.g. what Lynis's SSH-7408 test is actually about) and must
// not be reported as "the" effective value for everyone. Pure — no I/O.
// Exported for internal/native, which parses the same "Key Value"
// config style for its findings.
func DirectiveValue(content, key string) string {
	lines := strings.Split(content, "\n")
	globalEnd := len(lines)
	if idx := firstMatchLineIndex(lines); idx >= 0 {
		globalEnd = idx
	}

	for _, line := range lines[:globalEnd] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], key) {
			return fields[1]
		}
	}
	return ""
}

// firstMatchLineIndex returns the index of the first uncommented,
// top-level `Match` directive line in lines, or -1 if there is none.
// Per OpenSSH's sshd_config(5) semantics, a `Match` line opens a
// conditional block that extends to the next `Match` line or end of
// file, so everything from that index onward is scoped rather than
// global.
func firstMatchLineIndex(lines []string) int {
	for i, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 1 && strings.EqualFold(fields[0], "Match") {
			return i
		}
	}
	return -1
}

// directiveInMatchBlock reports whether key is set (uncommented) inside
// a Match block anywhere in content, i.e. after the first top-level
// Match line. Pure — no I/O. Used to warn operators that a fix managing
// key's global value coexists with a scoped override it deliberately
// does not touch.
func directiveInMatchBlock(content, key string) bool {
	lines := strings.Split(content, "\n")
	idx := firstMatchLineIndex(lines)
	if idx < 0 {
		return false
	}
	for _, line := range lines[idx:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], key) {
			return true
		}
	}
	return false
}
