package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Prompt prints label with its current default shown in brackets and reads
// one line of response from in. An empty response (bare Enter) resolves to
// def; any other line is returned trimmed of surrounding whitespace. An
// empty def shows as "(none)" so the prompt still reads sensibly.
//
// in must be the same *bufio.Reader shared across every Prompt and Confirm
// call against one underlying stream — see Confirm for why a fresh reader
// per call drops answers that arrived in the same read.
func Prompt(in *bufio.Reader, out io.Writer, label, def string) string {
	shown := def
	if shown == "" {
		shown = "(none)"
	}
	_, _ = fmt.Fprintf(out, "%s %s [%s] ", LabelInfo.Render("?"), label, shown)

	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}
