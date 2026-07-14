package lynis

import (
	"errors"
	"os/exec"
	"testing"
)

func TestLynisPathWithPrefersPATH(t *testing.T) {
	lookPath := func(name string) (string, error) { return "/custom/path/lynis", nil }
	exists := func(string) bool { t.Fatal("should not check fallbacks when PATH lookup succeeds"); return false }

	got, err := lynisPathWith(lookPath, exists, sbinFallbacks)
	if err != nil {
		t.Fatalf("lynisPathWith: %v", err)
	}
	if got != "/custom/path/lynis" {
		t.Errorf("got %q, want %q", got, "/custom/path/lynis")
	}
}

func TestLynisPathWithFallsBackToSbin(t *testing.T) {
	lookPath := func(name string) (string, error) { return "", exec.ErrNotFound }
	exists := func(p string) bool { return p == "/sbin/lynis" }

	got, err := lynisPathWith(lookPath, exists, []string{"/usr/sbin/lynis", "/sbin/lynis"})
	if err != nil {
		t.Fatalf("lynisPathWith: %v", err)
	}
	if got != "/sbin/lynis" {
		t.Errorf("got %q, want %q", got, "/sbin/lynis")
	}
}

func TestLynisPathWithErrorsWhenNowhereFound(t *testing.T) {
	lookPath := func(name string) (string, error) { return "", exec.ErrNotFound }
	exists := func(string) bool { return false }

	_, err := lynisPathWith(lookPath, exists, sbinFallbacks)
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("got %v, want exec.ErrNotFound", err)
	}
}
