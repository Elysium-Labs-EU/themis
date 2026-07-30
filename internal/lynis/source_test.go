package lynis

import (
	"os"
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
)

func TestSourceName(t *testing.T) {
	s := NewSource(Options{})
	if s.Name() != "lynis" {
		t.Errorf("Name() = %q, want %q", s.Name(), "lynis")
	}
}

func TestSourceRunErrorsWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; the requires-root guard can't be exercised")
	}
	s := NewSource(Options{})
	if _, err := s.Run(t.Context()); err == nil {
		t.Fatal("expected Run to error when not running as root")
	}
}

func TestRegisteredFactorySkipsWhenDisabled(t *testing.T) {
	sources, err := audit.Enabled(audit.SourceConfig{LynisEnabled: false})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	for _, s := range sources {
		if s.Name() == "lynis" {
			t.Fatal("lynis should be left out of the enabled set when LynisEnabled is false")
		}
	}
}

func TestRegisteredFactoryEnablesWhenEnabled(t *testing.T) {
	sources, err := audit.Enabled(audit.SourceConfig{LynisEnabled: true, LynisQuick: true})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	found := false
	for _, s := range sources {
		if s.Name() == "lynis" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected lynis to be enabled when LynisEnabled is true")
	}
}
