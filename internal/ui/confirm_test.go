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

// TestConfirmSharedReaderAnswersBothPrompts is a regression test for issue
// #26: two Confirm calls sharing one *bufio.Reader over piped input
// ("y\ny\n") must both see their answer. Before the fix, Confirm wrapped a
// fresh bufio.Reader per call, so the first call's read-ahead silently
// swallowed the second answer.
func TestConfirmSharedReaderAnswersBothPrompts(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("y\ny\n"))
	out := &bytes.Buffer{}

	first := Confirm(in, out, "remove binary?", false)
	second := Confirm(in, out, "remove state?", false)

	if !first || !second {
		t.Errorf("Confirm sequence = (%v, %v), want (true, true)", first, second)
	}
}
