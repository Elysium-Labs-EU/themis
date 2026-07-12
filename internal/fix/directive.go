package fix

import "strings"

// setDirective ensures a "Key Value" style config (one directive per
// line, '#' comments) has key set to value: comments out other
// uncommented lines for key and appends the desired line if it isn't
// already the last effective setting. Pure — no I/O.
func setDirective(content, key, value string) string {
	lines := strings.Split(content, "\n")
	// A trailing newline produces one empty trailing element; drop it so
	// appending the desired directive doesn't leave a blank line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines)+1)
	found := false
	for _, line := range lines {
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
		out = append(out, key+" "+value)
	}
	return strings.Join(out, "\n")
}

// directiveValue returns the last effective (uncommented) value for key,
// or "" if it is never set. Pure — no I/O.
func directiveValue(content, key string) string {
	value := ""
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], key) {
			value = fields[1]
		}
	}
	return value
}
