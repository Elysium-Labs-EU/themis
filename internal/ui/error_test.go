package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestUserErrorErrorReturnsWrappedMessage(t *testing.T) {
	e := &UserError{Err: errors.New("something failed")}
	if e.Error() != "something failed" {
		t.Errorf("Error() = %q, want %q", e.Error(), "something failed")
	}
}

func TestUserErrorUnwrapExposesWrappedErr(t *testing.T) {
	inner := errors.New("root cause")
	e := &UserError{Err: inner}
	if !errors.Is(e, inner) {
		t.Error("expected errors.Is to see through UserError to the wrapped error")
	}
}

func TestUserErrorRenderIncludesHintWhenSet(t *testing.T) {
	e := &UserError{Err: errors.New("themis check requires root"), Hint: "sudo themis check"}
	out := e.Render()
	if !strings.Contains(out, "themis check requires root") {
		t.Errorf("Render() = %q, want it to contain the error message", out)
	}
	if !strings.Contains(out, "sudo themis check") {
		t.Errorf("Render() = %q, want it to contain the hint", out)
	}
}

func TestUserErrorRenderOmitsHintLineWhenUnset(t *testing.T) {
	e := &UserError{Err: errors.New("boom")}
	out := e.Render()
	if strings.Contains(out, "run:") {
		t.Errorf("Render() = %q, want no hint line when Hint is empty", out)
	}
}
