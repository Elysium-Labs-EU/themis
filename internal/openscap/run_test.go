package openscap

import (
	"errors"
	"os"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/ui"
)

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOscapArgsNoProfile(t *testing.T) {
	got := oscapArgs(Options{ContentPath: "/content.xml"})
	want := []string{"xccdf", "eval", "/content.xml"}
	if !equalArgs(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOscapArgsWithProfile(t *testing.T) {
	got := oscapArgs(Options{ContentPath: "/content.xml", Profile: "xccdf_org.ssgproject.content_profile_cis"})
	want := []string{"xccdf", "eval", "--profile", "xccdf_org.ssgproject.content_profile_cis", "/content.xml"}
	if !equalArgs(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAuditErrorsWithoutContentPath(t *testing.T) {
	_, err := Audit(t.Context(), Options{})
	if err == nil {
		t.Fatal("expected an error when ContentPath is empty")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
	if uerr.Hint == "" {
		t.Error("expected a hint pointing at scap-security-guide")
	}
}

func TestAuditErrorsWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; skipping the non-root path")
	}

	_, err := Audit(t.Context(), Options{ContentPath: "/content.xml"})
	if err == nil {
		t.Fatal("expected an error when not running as root")
	}
	var uerr *ui.UserError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected a *ui.UserError in the chain, got %v", err)
	}
}

func TestRunOscapEvalErrorsWhenBinaryMissing(t *testing.T) {
	// A path that doesn't exist fails to even start (not an *exec.ExitError),
	// which must surface as a plain error rather than being swallowed the
	// way a non-zero exit (rule failures) is.
	_, err := runOscapEval(t.Context(), "/nonexistent/oscap-binary", Options{ContentPath: "/content.xml"})
	if err == nil {
		t.Fatal("expected an error when the oscap binary can't be started")
	}
}
