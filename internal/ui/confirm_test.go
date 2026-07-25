package ui

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"explicit yes", "y\n", false, true},
		{"explicit yes long form", "yes\n", false, true},
		{"explicit no overrides default yes", "n\n", true, false},
		{"empty response uses default (yes)", "\n", true, true},
		{"empty response uses default (no)", "\n", false, false},
		{"case insensitive", "Y\n", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := bufio.NewReader(strings.NewReader(tt.input))
			out := &bytes.Buffer{}
			got := Confirm(in, out, "proceed?", tt.defaultYes)
			if got != tt.want {
				t.Errorf("Confirm(%q, defaultYes=%v) = %v, want %v", tt.input, tt.defaultYes, got, tt.want)
			}
			if out.Len() == 0 {
				t.Error("expected a prompt to be written to out")
			}
		})
	}
}

// TestConfirmSharedReaderTwoPrompts reproduces issue #26: bufio.Reader reads
// ahead past the first "\n" on its underlying Read, so when both answers
// arrive in one shot (piped stdin), a fresh bufio.Reader per Confirm call
// buffers and then discards the second answer. Passing one shared
// *bufio.Reader across both calls must see both answers.
func TestConfirmSharedReaderTwoPrompts(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("y\ny\n"))
	out := &bytes.Buffer{}

	first := Confirm(in, out, "remove binary?", false)
	second := Confirm(in, out, "remove data?", false)

	if !first || !second {
		t.Errorf("Confirm(shared reader) = (%v, %v), want (true, true)", first, second)
	}
}
