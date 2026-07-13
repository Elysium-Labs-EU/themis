package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/Elysium_Labs/themis/internal/release"
)

// withFakeCodebergAPI points internal/release's Codeberg API base at a local
// httptest server for the duration of the test.
func withFakeCodebergAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Cleanup(release.SetAPIBase(srv.URL))
	return srv
}

func TestRunUpdateAlreadyLatest(t *testing.T) {
	withFakeCodebergAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "v0.1.0", "assets": []}`))
	})

	exePath := filepath.Join(t.TempDir(), "themis")
	buf := &bytes.Buffer{}

	if err := runUpdate(context.Background(), buf, exePath, "v0.1.0"); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(buf.String(), "already on the latest version") {
		t.Errorf("output = %q, want an already-latest message", buf.String())
	}
}

func TestRunUpdateNewerAvailableButNoMatchingAsset(t *testing.T) {
	withFakeCodebergAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "v9.9.9", "assets": []}`))
	})

	exePath := filepath.Join(t.TempDir(), "themis")
	buf := &bytes.Buffer{}

	err := runUpdate(context.Background(), buf, exePath, "v0.1.0")
	if err == nil {
		t.Fatal("expected an error when the release has no matching asset")
	}
	if !strings.Contains(err.Error(), "no asset for linux-") {
		t.Errorf("error = %v, want a missing-asset message", err)
	}
}

func TestNormalizeSemver(t *testing.T) {
	tests := map[string]string{
		"0.1.0":  "v0.1.0",
		"v0.1.0": "v0.1.0",
		"":       "",
	}
	for in, want := range tests {
		if got := normalizeSemver(in); got != want {
			t.Errorf("normalizeSemver(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHostArch(t *testing.T) {
	arch, err := hostArch()
	if err != nil {
		// Only amd64/arm64 are supported; this test environment may not be
		// one of them, which is itself a valid (if untested-further) path.
		t.Skipf("hostArch: %v", err)
	}
	if arch != "amd64" && arch != "arm64" {
		t.Errorf("hostArch() = %q, want amd64 or arm64", arch)
	}
}
