package ui

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestPromptReturnsDefaultOnEmptyLine(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("\n"))
	out := &bytes.Buffer{}

	got := Prompt(in, out, "label", "fallback")
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
	if !strings.Contains(out.String(), "label") || !strings.Contains(out.String(), "fallback") {
		t.Errorf("prompt text = %q, want it to mention label and the shown default", out.String())
	}
}

func TestPromptReturnsTrimmedResponse(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("  custom value  \n"))
	out := &bytes.Buffer{}

	got := Prompt(in, out, "label", "fallback")
	if got != "custom value" {
		t.Errorf("got %q, want %q", got, "custom value")
	}
}

func TestPromptShowsNoneForEmptyDefault(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("\n"))
	out := &bytes.Buffer{}

	Prompt(in, out, "label", "")
	if !strings.Contains(out.String(), "(none)") {
		t.Errorf("prompt text = %q, want it to show (none) for an empty default", out.String())
	}
}

func TestPromptReadsOneLineAcrossMultipleCalls(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("first\nsecond\n"))
	out := &bytes.Buffer{}

	if got := Prompt(in, out, "a", ""); got != "first" {
		t.Errorf("first call got %q, want %q", got, "first")
	}
	if got := Prompt(in, out, "b", ""); got != "second" {
		t.Errorf("second call got %q, want %q", got, "second")
	}
}
