package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/Elysium_Labs/themis/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.0.2",
			"assets": [
				{"name": "themis-linux-amd64", "browser_download_url": "https://example.com/themis-linux-amd64"},
				{"name": "sha256sums.txt", "browser_download_url": "https://example.com/sha256sums.txt"}
			]
		}`))
	}))
	defer srv.Close()

	origBase := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = origBase })

	rel, err := FetchLatest(context.Background(), "Elysium_Labs/themis")
	if err != nil {
		t.Fatalf("FetchLatest: %v", err)
	}
	if rel.TagName != "v0.0.2" {
		t.Errorf("TagName = %q, want %q", rel.TagName, "v0.0.2")
	}
	if len(rel.Assets) != 2 {
		t.Fatalf("Assets = %+v, want 2 entries", rel.Assets)
	}
}

func TestFetchLatestNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origBase := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = origBase })

	if _, err := FetchLatest(context.Background(), "Elysium_Labs/themis"); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func TestReleaseAssetFor(t *testing.T) {
	rel := Release{
		Assets: []Asset{
			{Name: "themis-linux-amd64", DownloadURL: "https://codeberg.org/x/amd64"},
			{Name: "themis-linux-arm64", DownloadURL: "https://codeberg.org/x/arm64"},
			{Name: "sha256sums.txt", DownloadURL: "https://codeberg.org/x/sums"},
		},
	}

	a, ok := rel.AssetFor("amd64")
	if !ok || a.Name != "themis-linux-amd64" {
		t.Fatalf("AssetFor(amd64) = %+v, %v", a, ok)
	}

	if _, matched := rel.AssetFor("mips"); matched {
		t.Fatal("AssetFor(mips) should not match")
	}

	sums, ok := rel.ChecksumsAsset()
	if !ok || sums.Name != "sha256sums.txt" {
		t.Fatalf("ChecksumsAsset() = %+v, %v", sums, ok)
	}
}

func TestDownloadRejectsUntrustedHost(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out")
	err := Download(context.Background(), "https://evil.example.com/themis", dst)
	if err == nil {
		t.Fatal("expected an error for a non-codeberg.org host")
	}
}

func TestDownloadRejectsNonHTTPS(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out")
	err := Download(context.Background(), "http://codeberg.org/themis", dst)
	if err == nil {
		t.Fatal("expected an error for a non-https scheme")
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "themis-linux-amd64")
	if err := os.WriteFile(binPath, []byte("fake binary contents"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sum, err := sha256File(binPath)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}

	checksums := sum + "  themis-linux-amd64\nother  other-file\n"
	if err := VerifyChecksum(binPath, checksums, "themis-linux-amd64"); err != nil {
		t.Errorf("VerifyChecksum with matching sum: %v", err)
	}

	if err := VerifyChecksum(binPath, "deadbeef  themis-linux-amd64\n", "themis-linux-amd64"); err == nil {
		t.Error("expected an error for a checksum mismatch")
	}

	if err := VerifyChecksum(binPath, "deadbeef  other-file\n", "themis-linux-amd64"); err == nil {
		t.Error("expected an error when no entry matches the asset name")
	}
}

func TestCopyFileAndReplaceBinary(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new-binary")
	if err := os.WriteFile(src, []byte("new contents"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dst := filepath.Join(dir, "installed-binary")
	if err := os.WriteFile(dst, []byte("old contents"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := ReplaceBinary(src, dst); err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new contents" {
		t.Errorf("dst contents = %q, want %q", got, "new contents")
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("dst is not executable: mode %v", info.Mode())
	}

	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected the .tmp file to be gone after rename, got err=%v", err)
	}
}

func TestCheckWritable(t *testing.T) {
	dir := t.TempDir()
	if err := CheckWritable(dir); err != nil {
		t.Errorf("CheckWritable(%s): %v", dir, err)
	}

	if err := CheckWritable(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Error("expected an error for a non-existent directory")
	}
}
