package ui

import (
	"errors"
	"testing"
)

// TestWithSpinnerRunsFnWhenStderrIsNotATerminal covers the non-interactive
// fast path — the only one exercisable under `go test`, since stderr is
// captured rather than a real terminal. WithSpinner must still call fn
// and propagate its result unchanged.
func TestWithSpinnerRunsFnWhenStderrIsNotATerminal(t *testing.T) {
	called := false
	err := WithSpinner("doing work", func() error {
		called = true
		return nil
	})
	if !called {
		t.Fatal("expected fn to run")
	}
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	wantErr := errors.New("boom")
	if got := WithSpinner("doing work", func() error { return wantErr }); !errors.Is(got, wantErr) {
		t.Fatalf("err = %v, want %v", got, wantErr)
	}
}
