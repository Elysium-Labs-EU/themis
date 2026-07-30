package openscap

import (
	"testing"

	"github.com/Elysium-Labs-EU/themis/internal/audit"
)

func TestSourceName(t *testing.T) {
	s := NewSource(Options{ContentPath: "/content.xml"})
	if s.Name() != "openscap" {
		t.Errorf("Name() = %q, want %q", s.Name(), "openscap")
	}
}

func TestSourceRunErrorsWithoutContentPath(t *testing.T) {
	s := NewSource(Options{})
	if _, err := s.Run(t.Context()); err == nil {
		t.Fatal("expected Run to error when ContentPath is empty")
	}
}

func TestRegisteredFactorySkipsWithoutContentPath(t *testing.T) {
	sources, err := audit.Enabled(audit.SourceConfig{})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	for _, s := range sources {
		if s.Name() == "openscap" {
			t.Fatal("openscap should be left out of the enabled set with no content path configured")
		}
	}
}

func TestRegisteredFactoryEnablesWithContentPath(t *testing.T) {
	sources, err := audit.Enabled(audit.SourceConfig{OpenSCAPContentPath: "/content.xml", OpenSCAPProfile: "cis"})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	found := false
	for _, s := range sources {
		if s.Name() == "openscap" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected openscap to be enabled once a content path is configured")
	}
}
