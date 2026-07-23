package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm prints a yes/no prompt to out and reads one line of response from
// in. An empty response (bare Enter) resolves to defaultYes.
//
// in must be a *bufio.Reader shared across every Confirm call against the
// same underlying stream: bufio.Reader reads ahead past the first "\n" in
// one Read, so wrapping a fresh bufio.Reader per call discards any input
// already buffered for a subsequent prompt.
func Confirm(in *bufio.Reader, out io.Writer, prompt string, defaultYes bool) bool {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	_, _ = fmt.Fprintf(out, "%s %s %s ", LabelWarning.Render("?"), prompt, suffix)

	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}
