//go:build integration

package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Integration test for the self-update path. It is HERMETIC — no external
// network — but exercises the REAL end-to-end runUpdate flow:
//
//	fetch release -> download asset + checksums -> verify checksum ->
//	back up current binary -> replace binary in place
//
// against a local httptest server (wired in via useHTTPTestServer, which
// rewrites the hardcoded github.com URLs to the test server). It needs no
// root and no special OS, so it also runs on the CI runner.
//
// HOME is redirected to a temp dir so runUpdate's completion-refresh step
// finds no installed completion scripts and never execs the fake binary.

// serveRelease returns a handler that answers the three requests runUpdate
// makes: the release metadata, the binary asset, and sha256sums.txt.
// binContent is served as the asset; sumsContent as the checksums file.
func serveRelease(t *testing.T, assetName, binContent, sumsContent string) http.HandlerFunc {
	t.Helper()
	assetURL := "https://github.com/dl/" + assetName
	sumsURL := "https://github.com/dl/sha256sums.txt"
	relJSON := fmt.Sprintf(`{
		"tag_name": "v99.0.0",
		"assets": [
			{"name": %q, "browser_download_url": %q},
			{"name": "sha256sums.txt", "browser_download_url": %q}
		]
	}`, assetName, assetURL, sumsURL)

	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(relJSON))
		case strings.HasSuffix(r.URL.Path, "/"+assetName):
			_, _ = w.Write([]byte(binContent))
		case strings.HasSuffix(r.URL.Path, "/sha256sums.txt"):
			_, _ = w.Write([]byte(sumsContent))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestRunUpdateEndToEndIntegration drives the full happy-path self-update
// against a temp binary and asserts the binary is replaced with the
// downloaded bytes and the prior binary is backed up.
func TestRunUpdateEndToEndIntegration(t *testing.T) {
	arch, err := hostArch()
	if err != nil {
		t.Skipf("unsupported arch for release asset naming: %v", err)
	}
	assetName := "themis-linux-" + arch

	const (
		oldBin = "OLD themis binary bytes"
		newBin = "NEW themis binary bytes v99"
	)
	sums := sha256Hex(newBin) + "  " + assetName + "\n"

	useHTTPTestServer(t, serveRelease(t, assetName, newBin, sums))
	t.Setenv("HOME", t.TempDir()) // sandbox completion-refresh

	dir := t.TempDir()
	exePath := filepath.Join(dir, "themis")
	if err := os.WriteFile(exePath, []byte(oldBin), 0o755); err != nil { //nolint:gosec // test binary needs to be executable to model reality
		t.Fatalf("seeding exe: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := runUpdate(context.Background(), buf, exePath, "0.0.1", false); err != nil {
		t.Fatalf("runUpdate: %v\noutput:\n%s", err, buf.String())
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("reading updated binary: %v", err)
	}
	if string(got) != newBin {
		t.Errorf("installed binary = %q, want the downloaded bytes %q", got, newBin)
	}

	backup, err := os.ReadFile(exePath + ".backup")
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backup) != oldBin {
		t.Errorf("backup = %q, want the prior bytes %q", backup, oldBin)
	}

	for _, want := range []string{"checksum verified", "updated 0.0.1 -> v99.0.0"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("output missing %q:\n%s", want, buf.String())
		}
	}
}

// TestRunUpdateRejectsBadChecksumIntegration proves a mismatched checksum
// aborts the update BEFORE the on-disk binary is touched.
func TestRunUpdateRejectsBadChecksumIntegration(t *testing.T) {
	arch, err := hostArch()
	if err != nil {
		t.Skipf("unsupported arch for release asset naming: %v", err)
	}
	assetName := "themis-linux-" + arch

	const (
		oldBin = "OLD themis binary bytes"
		newBin = "NEW themis binary bytes v99"
	)
	// Checksum for the WRONG content — verification must reject the download.
	badSums := sha256Hex("something else entirely") + "  " + assetName + "\n"

	useHTTPTestServer(t, serveRelease(t, assetName, newBin, badSums))
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	exePath := filepath.Join(dir, "themis")
	if err := os.WriteFile(exePath, []byte(oldBin), 0o755); err != nil { //nolint:gosec // test binary needs to be executable to model reality
		t.Fatalf("seeding exe: %v", err)
	}

	err = runUpdate(context.Background(), &bytes.Buffer{}, exePath, "0.0.1", false)
	if err == nil {
		t.Fatal("expected runUpdate to fail on a checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %v, want a checksum mismatch", err)
	}

	// The original binary must be untouched — no partial replace.
	got, readErr := os.ReadFile(exePath)
	if readErr != nil {
		t.Fatalf("reading binary after rejected update: %v", readErr)
	}
	if string(got) != oldBin {
		t.Errorf("binary changed despite bad checksum: got %q, want %q", got, oldBin)
	}
	if _, statErr := os.Stat(exePath + ".backup"); statErr == nil {
		t.Error("a backup was created even though the update was rejected before replace")
	}
}
