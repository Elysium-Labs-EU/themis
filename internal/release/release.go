// Package release fetches themis releases from Codeberg and performs the
// download/verify/swap steps needed to self-update the installed binary.
package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// apiBase is the Codeberg API root. It's a var (not a const) so tests can
// point FetchLatest at an httptest server instead of the real Codeberg.
var apiBase = "https://codeberg.org"

// SetAPIBase overrides the Codeberg API base URL and returns a func that
// restores the original. For tests only.
func SetAPIBase(base string) (restore func()) {
	orig := apiBase
	apiBase = base
	return func() { apiBase = orig }
}

// Asset is one file attached to a Codeberg release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Release is the subset of Codeberg's release API response themis needs.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// FetchLatest fetches the latest release for repo, e.g. "Elysium_Labs/themis".
func FetchLatest(ctx context.Context, repo string) (Release, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/releases/latest", apiBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("building release request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetching latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("fetching latest release: unexpected status %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("decoding release response: %w", err)
	}
	return rel, nil
}

// AssetFor returns the release asset for themis on linux/arch.
func (r Release) AssetFor(arch string) (Asset, bool) {
	want := fmt.Sprintf("themis-linux-%s", arch)
	for _, a := range r.Assets {
		if a.Name == want {
			return a, true
		}
	}
	return Asset{}, false
}

// ChecksumsAsset returns the sha256sums.txt asset, if the release has one.
func (r Release) ChecksumsAsset() (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == "sha256sums.txt" {
			return a, true
		}
	}
	return Asset{}, false
}

// Download fetches downloadURL to destPath. It refuses anything but a
// plain https://codeberg.org URL, since this is used to fetch and then
// execute-in-place a new themis binary.
func Download(ctx context.Context, downloadURL, destPath string) error {
	u, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("parsing download URL: %w", err)
	}
	if u.Scheme != "https" || u.Host != "codeberg.org" {
		return fmt.Errorf("refusing to download from untrusted host %q", u.Host)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("building download request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", downloadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %s", downloadURL, resp.Status)
	}

	out, err := os.Create(destPath) //nolint:gosec // destPath is a caller-controlled temp path
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer func() { _ = out.Close() }()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("writing %s: %w", destPath, err)
	}
	if resp.ContentLength > 0 && n != resp.ContentLength {
		return fmt.Errorf("downloading %s: got %d bytes, expected %d", downloadURL, n, resp.ContentLength)
	}
	return nil
}

// VerifyChecksum checks binaryPath's sha256 against the entry for assetName
// in a sha256sums.txt file's contents (the standard `sha256sum` output
// format: "<hex digest>  <filename>" per line).
func VerifyChecksum(binaryPath, checksumsContent, assetName string) error {
	var want string
	for _, line := range strings.Split(checksumsContent, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum entry for %s", assetName)
	}

	f, err := os.Open(binaryPath) //nolint:gosec // binaryPath is a caller-controlled temp path
	if err != nil {
		return fmt.Errorf("opening %s: %w", binaryPath, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing %s: %w", binaryPath, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, want, got)
	}
	return nil
}

// CopyFile copies src to dst, creating or truncating dst, preserving src's
// file mode.
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	in, err := os.Open(src) //nolint:gosec // caller-controlled paths
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying to %s: %w", dst, err)
	}
	return nil
}

// ReplaceBinary installs newPath over dstPath, which may be the currently
// running executable: it copies to a same-directory temp file, chmods it
// executable, then renames over dstPath. The rename is atomic on the same
// filesystem, and the OS keeps the old inode open for any process (e.g.
// the one calling this function) that's already running it.
func ReplaceBinary(newPath, dstPath string) error {
	tmp := dstPath + ".tmp"
	if err := CopyFile(newPath, tmp); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil { //nolint:gosec // installed binary must be executable
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("installing %s: %w", dstPath, err)
	}
	return nil
}

// CheckWritable verifies dir is writable by creating and removing a probe
// file in it.
func CheckWritable(dir string) error {
	probe := filepath.Join(dir, ".themis-write-check")
	f, err := os.Create(probe) //nolint:gosec // fixed probe filename in a caller-controlled dir
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(probe)
}
