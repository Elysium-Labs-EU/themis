package audit

import (
	"context"
	"errors"
	"testing"
)

// fakeSource is a Source a test registers by name to prove the registry
// seam: it is constructed solely through Register/Enabled, with no edit
// to any cmd package.
type fakeSource struct {
	name     string
	findings []Finding
}

func (f fakeSource) Name() string { return f.name }

func (f fakeSource) Run(context.Context) ([]Finding, error) { return f.findings, nil }

// withCleanRegistry saves and restores the process-wide registry so a
// test can register sources without leaking them into other tests.
func withCleanRegistry(t *testing.T) {
	t.Helper()
	saved := registry
	registry = nil
	t.Cleanup(func() { registry = saved })
}

// TestSourceConstructedByNameFlowsThroughRun is the acceptance test from
// issue #81: a source registered purely by name shows up in audit.Run's
// output when built via Enabled, with no command-layer wiring.
func TestSourceConstructedByNameFlowsThroughRun(t *testing.T) {
	withCleanRegistry(t)

	want := Finding{TestID: "FAKE-1", Kind: "warning", Description: "fake finding", Source: "fake"}
	Register("fake", 10, func(SourceConfig) (Source, error) {
		return fakeSource{name: "fake", findings: []Finding{want}}, nil
	})

	srcs, err := Enabled(SourceConfig{})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	if len(srcs) != 1 {
		t.Fatalf("expected 1 enabled source, got %d", len(srcs))
	}

	findings, err := Run(context.Background(), srcs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 || findings[0] != want {
		t.Fatalf("expected the fake source's finding in Run output, got %+v", findings)
	}
}

// TestEnabledOrdersByRegistrationOrder checks that Enabled returns
// sources ascending by their registered order, independent of the order
// they were registered in.
func TestEnabledOrdersByRegistrationOrder(t *testing.T) {
	withCleanRegistry(t)

	Register("third", 30, func(SourceConfig) (Source, error) { return fakeSource{name: "third"}, nil })
	Register("first", 10, func(SourceConfig) (Source, error) { return fakeSource{name: "first"}, nil })
	Register("second", 20, func(SourceConfig) (Source, error) { return fakeSource{name: "second"}, nil })

	srcs, err := Enabled(SourceConfig{})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	got := []string{}
	for _, s := range srcs {
		got = append(got, s.Name())
	}
	want := []string{"first", "second", "third"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("enabled order = %v, want %v", got, want)
		}
	}
}

// TestEnabledSkipsFactoriesReturningNotEnabled confirms a factory that
// opts out for a given config (ErrSourceNotEnabled) is left out of the
// enabled set entirely — how openscap stays out when no content path is
// set.
func TestEnabledSkipsFactoriesReturningNotEnabled(t *testing.T) {
	withCleanRegistry(t)

	Register("always", 10, func(SourceConfig) (Source, error) { return fakeSource{name: "always"}, nil })
	Register("optin", 20, func(cfg SourceConfig) (Source, error) {
		if cfg.OpenSCAPContentPath == "" {
			return nil, ErrSourceNotEnabled
		}
		return fakeSource{name: "optin"}, nil
	})

	srcs, err := Enabled(SourceConfig{})
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	if len(srcs) != 1 || srcs[0].Name() != "always" {
		t.Fatalf("expected only the always-on source, got %+v", srcs)
	}

	srcs, err = Enabled(SourceConfig{OpenSCAPContentPath: "/some/content.xml"})
	if err != nil {
		t.Fatalf("Enabled with content: %v", err)
	}
	if len(srcs) != 2 {
		t.Fatalf("expected both sources enabled when opt-in config set, got %+v", srcs)
	}
}

// TestEnabledPropagatesFactoryError checks a factory error aborts the
// whole build, wrapped with the source name.
func TestEnabledPropagatesFactoryError(t *testing.T) {
	withCleanRegistry(t)

	boom := errors.New("boom")
	Register("broken", 10, func(SourceConfig) (Source, error) { return nil, boom })

	_, err := Enabled(SourceConfig{})
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped factory error, got %v", err)
	}
}

// TestRegisterPanicsOnDuplicateName guards the wiring invariant that two
// sources never share a name.
func TestRegisterPanicsOnDuplicateName(t *testing.T) {
	withCleanRegistry(t)

	Register("dup", 10, func(SourceConfig) (Source, error) { return fakeSource{name: "dup"}, nil })

	defer func() {
		if recover() == nil {
			t.Fatal("expected Register to panic on a duplicate name")
		}
	}()
	Register("dup", 20, func(SourceConfig) (Source, error) { return fakeSource{name: "dup"}, nil })
}
