package cmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Elysium-Labs-EU/themis/internal/buildinfo"
	"github.com/Elysium-Labs-EU/themis/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const themisRepo = "Elysium-Labs-EU/themis"

// userAgent is sent on every GitHub API/download request. GitHub's REST API
// rejects requests with no User-Agent (Gitea/Codeberg did not require one).
const userAgent = "themis-updater"

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

// Asset is one file attached to a GitHub release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Release is the subset of GitHub's release API response themis needs.
type Release struct {
	TagName    string  `json:"tag_name"`
	Assets     []Asset `json:"assets"`
	Prerelease bool    `json:"prerelease"`
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

// SignatureAsset returns the sha256sums.txt.sig asset — a detached ECDSA
// signature over sha256sums.txt produced at release time by
// .github/workflows/release.yml — if the release has one. Releases
// published before signing was introduced won't have one; see
// requireReleaseSignature.
func (r Release) SignatureAsset() (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == "sha256sums.txt.sig" {
			return a, true
		}
	}
	return Asset{}, false
}

// fetchLatestRelease fetches the release to install from GitHub.
//
// Without includePre, this hits GitHub's "latest" endpoint, which only ever
// returns a stable (non-prerelease) release. If that 404s — which happens
// whenever every published release is currently a prerelease — it falls
// back to listing all releases and picking one via pickLatestRelease.
//
// With includePre, this always lists all releases and picks via
// pickLatestRelease, since GitHub's release list is ordered by creation
// time, not version, and has been observed to return a release out of
// order.
func fetchLatestRelease(ctx context.Context, includePre bool) (Release, error) {
	if !includePre {
		rel, notFound, err := fetchLatestStableRelease(ctx)
		if err != nil {
			return Release{}, err
		}
		if !notFound {
			return rel, nil
		}
	}

	releases, err := fetchReleaseList(ctx)
	if err != nil {
		return Release{}, err
	}
	return pickLatestRelease(releases)
}

// newGitHubAPIRequest builds a GET request against reqURL with the headers
// GitHub's REST API requires.
func newGitHubAPIRequest(ctx context.Context, reqURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building release request: %w", err)
	}
	// GitHub's API 403s requests with no User-Agent; Accept pins the API version.
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	return req, nil
}

// fetchLatestStableRelease hits GitHub's /releases/latest endpoint, which
// only ever returns a stable release. notFound reports a 404 response
// distinctly from other errors, since it's the signal that every published
// release is currently a prerelease.
func fetchLatestStableRelease(ctx context.Context) (rel Release, notFound bool, err error) {
	reqURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", themisRepo)
	req, err := newGitHubAPIRequest(ctx, reqURL)
	if err != nil {
		return Release{}, false, err
	}

	resp, err := httpClient.Do(req) // #nosec G704 -- URL is constructed from a hardcoded GitHub API base, not user input
	if err != nil {
		return Release{}, false, fmt.Errorf("fetching latest release: %w", err)
	}
	if resp == nil {
		return Release{}, false, fmt.Errorf("fetching latest release: nil response")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return Release{}, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("fetching latest release: unexpected status %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, false, fmt.Errorf("decoding release response: %w", err)
	}
	return rel, false, nil
}

// fetchReleaseList fetches every published release from GitHub — including
// prereleases — in whatever order GitHub returns them.
func fetchReleaseList(ctx context.Context) ([]Release, error) {
	reqURL := fmt.Sprintf("https://api.github.com/repos/%s/releases", themisRepo)
	req, err := newGitHubAPIRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req) // #nosec G704 -- URL is constructed from a hardcoded GitHub API base, not user input
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("fetching releases: nil response")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching releases: unexpected status %s", resp.Status)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding release response: %w", err)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	return releases, nil
}

// pickLatestRelease selects the release to install from releases, which may
// be in any order: the highest stable (non-prerelease) tag by semver, or —
// only if none of releases is stable — the highest prerelease tag. Releases
// whose tag isn't valid semver are ignored. Pure — no I/O.
func pickLatestRelease(releases []Release) (Release, error) {
	var bestStable, bestPre Release
	haveStable, havePre := false, false

	for _, r := range releases {
		tag := normalizeSemver(r.TagName)
		if !semver.IsValid(tag) {
			continue
		}
		if r.Prerelease {
			if !havePre || semver.Compare(tag, normalizeSemver(bestPre.TagName)) > 0 {
				bestPre, havePre = r, true
			}
			continue
		}
		if !haveStable || semver.Compare(tag, normalizeSemver(bestStable.TagName)) > 0 {
			bestStable, haveStable = r, true
		}
	}

	if haveStable {
		return bestStable, nil
	}
	if havePre {
		return bestPre, nil
	}
	return Release{}, fmt.Errorf("no releases with a valid semver tag found")
}

// releaseSigningPublicKeyPEM is the ECDSA P-256 public key (SubjectPublicKeyInfo,
// PEM) used to verify the detached signature over each release's
// sha256sums.txt. The matching private key lives only as the
// RELEASE_SIGNING_KEY secret in GitHub Actions and is used by
// .github/workflows/release.yml to sign at release time — it is never
// checked into this repo.
const releaseSigningPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZo6eWxjF1xhHMI/MyUNptSdkxuHM
qAeiDXd1PrPNR3I1N1radAb1df3CPt0WjZQmuTesJLQiDL91WwVt7fraSA==
-----END PUBLIC KEY-----
`

// requireReleaseSignature gates whether a release with no sha256sums.txt.sig
// asset is refused outright rather than merely warned about. Keep this false
// until the RELEASE_SIGNING_KEY secret is provisioned in GitHub Actions
// and the first signed release has shipped — flipping it before then would
// make every existing release (and install.sh, which tracks main) refuse to
// install. Once a signed release exists, flip to true so an unsigned or
// signature-stripped release can no longer be installed silently.
//
// Deliberately NOT flipped as part of the issue #29 signing-integrity fix
// (install.sh's stale embedded key, see releaseSigningPublicKeyPEM below):
// that task closed the key-drift gap and verified it against the current
// v0.0.3-rc.1 release's signature, but flipping this to a hard fail is a
// separate human rollout decision — it should only happen once a real
// tagged release has been verified end-to-end against the current key by
// someone deliberately deciding to make unsigned releases fatal, not as a
// side effect of a drift fix.
const requireReleaseSignature = false

// allowedDownloadHost is the only host themis will fetch an executable from.
const allowedDownloadHost = "github.com"

// validateDownloadURL refuses anything but a plain https://github.com URL,
// since downloadFile fetches and then executes-in-place a new themis binary.
// Pure — no I/O.
func validateDownloadURL(downloadURL string) error {
	u, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("parsing download URL: %w", err)
	}
	if u.Scheme != "https" || u.Host != allowedDownloadHost {
		return fmt.Errorf("refusing to download from untrusted host %q", u.Host)
	}
	return nil
}

// downloadFile fetches downloadURL to destPath, refusing any host but
// https://github.com (see validateDownloadURL).
func downloadFile(ctx context.Context, downloadURL, destPath string) error {
	if err := validateDownloadURL(downloadURL); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("building download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req) // #nosec G704 -- downloadURL is validated above to be https://github.com
	if err != nil {
		return fmt.Errorf("downloading %s: %w", downloadURL, err)
	}
	if resp == nil {
		return fmt.Errorf("downloading %s: nil response", downloadURL)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %s", downloadURL, resp.Status)
	}

	return writeResponseBody(destPath, resp, downloadURL)
}

// writeResponseBody streams resp.Body to destPath, verifying the byte count
// against Content-Length when the server provides one.
func writeResponseBody(destPath string, resp *http.Response, downloadURL string) error {
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

// verifyChecksum checks binaryPath's sha256 against the entry for assetName
// in a sha256sums.txt file's contents (the standard `sha256sum` output
// format: "<hex digest>  <filename>" per line).
func verifyChecksum(binaryPath, checksumsContent, assetName string) error {
	var want string
	for line := range strings.SplitSeq(checksumsContent, "\n") {
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

// parseReleaseSigningPublicKey decodes the embedded release signing public
// key. Pure — no I/O.
func parseReleaseSigningPublicKey() (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(releaseSigningPublicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("decoding embedded release signing public key: no PEM block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing embedded release signing public key: %w", err)
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("embedded release signing public key is %T, want ECDSA", pub)
	}
	return ecdsaPub, nil
}

// verifySignature checks sig — an ASN.1 DER ECDSA signature, as produced by
// `openssl dgst -sha256 -sign` — against the SHA-256 digest of data, using
// pub. Pure — no I/O.
func verifySignature(pub *ecdsa.PublicKey, data, sig []byte) error {
	digest := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return fmt.Errorf("signature does not match")
	}
	return nil
}

// verifyChecksumsSignature checks sig against checksumsData using the
// embedded release signing public key. Pure — no I/O.
func verifyChecksumsSignature(checksumsData, sig []byte) error {
	pub, err := parseReleaseSigningPublicKey()
	if err != nil {
		return err
	}
	if err := verifySignature(pub, checksumsData, sig); err != nil {
		return fmt.Errorf("signature does not match sha256sums.txt")
	}
	return nil
}

// verifyReleaseSignature downloads rel's sha256sums.txt.sig into tmpDir and
// verifies it against checksumsData, writing a status line to out either
// way.
//
// A release with no signature asset is only a hard error once
// requireReleaseSignature is true (see its doc comment for the rollout
// plan); until then it's a warning, since sha256 checksum verification
// alone has already run by the time this is called. A signature asset that
// fails to verify is always a hard error — that's a stronger integrity
// signal than "no signature was ever published", so it's never soft-failed.
func verifyReleaseSignature(ctx context.Context, out io.Writer, rel Release, checksumsData []byte, tmpDir string) error {
	sigAsset, ok := rel.SignatureAsset()
	if !ok {
		if requireReleaseSignature {
			return &ui.UserError{Err: fmt.Errorf("release %s has no sha256sums.txt.sig", rel.TagName)}
		}
		_, _ = fmt.Fprintf(out, "%s release %s has no signature (sha256sums.txt.sig) — checksum-only integrity\n", ui.LabelWarning.Render("warning"), rel.TagName)
		return nil
	}

	sigTmp := filepath.Join(tmpDir, "sha256sums.txt.sig")
	if dlErr := downloadFile(ctx, sigAsset.DownloadURL, sigTmp); dlErr != nil {
		return fmt.Errorf("downloading signature: %w", dlErr)
	}
	sigData, err := os.ReadFile(sigTmp) //nolint:gosec // fixed name in a themis-owned temp dir
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	if verifyErr := verifyChecksumsSignature(checksumsData, sigData); verifyErr != nil {
		return &ui.UserError{Err: fmt.Errorf("signature verification failed for %s: %w — refusing to install", rel.TagName, verifyErr)}
	}
	_, _ = fmt.Fprintf(out, "%s signature verified\n", ui.LabelSuccess.Render("✓"))
	return nil
}

// copyFile copies src to dst, creating or truncating dst, preserving src's
// file mode.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	in, err := os.Open(src) //nolint:gosec // caller-controlled paths
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode()) //nolint:gosec // caller-controlled paths
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying to %s: %w", dst, err)
	}
	return nil
}

// replaceBinary installs newPath over dstPath, which may be the currently
// running executable: it copies to a same-directory temp file, chmods it
// executable, then renames over dstPath. The rename is atomic on the same
// filesystem, and the OS keeps the old inode open for any process (e.g.
// the one calling this function) that's already running it.
func replaceBinary(newPath, dstPath string) error {
	tmp := dstPath + ".tmp"
	if err := copyFile(newPath, tmp); err != nil {
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

// checkWritable verifies dir is writable by creating and removing a probe
// file in it.
func checkWritable(dir string) error {
	probe := filepath.Join(dir, ".themis-write-check")
	f, err := os.Create(probe) //nolint:gosec // fixed probe filename in a caller-controlled dir
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(probe)
}

// currentBinaryPath returns the resolved path of the running themis binary.
func currentBinaryPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating running binary: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("resolving running binary path: %w", err)
	}
	return exePath, nil
}

// hostArch maps runtime.GOARCH to the arch suffix used in release asset
// names (see install.sh's detect_arch).
func hostArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return runtime.GOARCH, nil
	default:
		return "", &ui.UserError{Err: fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)}
	}
}

// normalizeSemver prefixes a bare "0.0.1"-style version with "v" so it's
// valid input for golang.org/x/mod/semver, which requires the "v" prefix.
func normalizeSemver(v string) string {
	if v != "" && v[0] != 'v' {
		return "v" + v
	}
	return v
}

// isUpToDate reports whether currentVersion is already at or ahead of
// latestVer. Both must be valid semver for the comparison to apply; if
// either isn't, an update is assumed available. Pure — no I/O.
func isUpToDate(currentVersion, latestVer string) bool {
	currentVer := normalizeSemver(currentVersion)
	return semver.IsValid(currentVer) && semver.IsValid(latestVer) &&
		semver.Compare(currentVer, latestVer) >= 0
}

// resolveReleaseAssets picks the themis binary asset for linux/arch and the
// checksums asset from rel, erroring (as a UserError) if either is absent.
// Pure — no I/O.
func resolveReleaseAssets(rel Release, arch string) (binary, checksums Asset, err error) {
	binary, ok := rel.AssetFor(arch)
	if !ok {
		return Asset{}, Asset{}, &ui.UserError{Err: fmt.Errorf("release %s has no asset for linux-%s", rel.TagName, arch)}
	}
	checksums, ok = rel.ChecksumsAsset()
	if !ok {
		return Asset{}, Asset{}, &ui.UserError{Err: fmt.Errorf("release %s is missing sha256sums.txt", rel.TagName)}
	}
	return binary, checksums, nil
}

// downloadVerifiedBinary downloads the release binary and its checksums into
// tmpDir, verifies the binary's sha256 and (see verifyReleaseSignature) the
// checksums' signature, and returns the local binary path.
func downloadVerifiedBinary(ctx context.Context, out io.Writer, rel Release, binary, checksums Asset, tmpDir, latestVer string) (string, error) {
	binTmp := filepath.Join(tmpDir, "themis")
	err := ui.WithSpinner(fmt.Sprintf("Downloading %s...", latestVer), func() error {
		return downloadFile(ctx, binary.DownloadURL, binTmp)
	})
	if err != nil {
		return "", fmt.Errorf("downloading update: %w", err)
	}

	checksumsTmp := filepath.Join(tmpDir, "sha256sums.txt")
	if dlErr := downloadFile(ctx, checksums.DownloadURL, checksumsTmp); dlErr != nil {
		return "", fmt.Errorf("downloading checksums: %w", dlErr)
	}
	checksumsData, err := os.ReadFile(checksumsTmp) //nolint:gosec // fixed name in a themis-owned temp dir
	if err != nil {
		return "", fmt.Errorf("reading checksums: %w", err)
	}
	if verifyErr := verifyChecksum(binTmp, string(checksumsData), binary.Name); verifyErr != nil {
		return "", &ui.UserError{Err: verifyErr}
	}
	_, _ = fmt.Fprintf(out, "%s checksum verified\n", ui.LabelSuccess.Render("✓"))

	if sigErr := verifyReleaseSignature(ctx, out, rel, checksumsData, tmpDir); sigErr != nil {
		return "", sigErr
	}
	return binTmp, nil
}

// runUpdate implements `themis system update` against an explicit exePath, so it
// can be exercised in tests without touching the test binary itself
// (os.Executable() under `go test` is the test binary).
func runUpdate(ctx context.Context, out io.Writer, exePath, currentVersion string, includePre bool) error {
	var rel Release
	err := ui.WithSpinner("Checking for updates...", func() error {
		var err error
		rel, err = fetchLatestRelease(ctx, includePre)
		return err
	})
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	latestVer := rel.TagName
	if isUpToDate(currentVersion, latestVer) {
		_, _ = fmt.Fprintf(out, "%s already on the latest version (%s)\n", ui.LabelSuccess.Render("✓"), currentVersion)
		return nil
	}

	_, _ = fmt.Fprintf(out, "%s new version available: %s -> %s\n", ui.LabelInfo.Render("i"), currentVersion, latestVer)

	arch, err := hostArch()
	if err != nil {
		return err
	}
	binary, checksums, err := resolveReleaseAssets(rel, arch)
	if err != nil {
		return err
	}

	if writeErr := checkWritable(filepath.Dir(exePath)); writeErr != nil {
		return &ui.UserError{
			Err:  fmt.Errorf("%s is not writable: %w", filepath.Dir(exePath), writeErr),
			Hint: "sudo themis system update",
		}
	}

	tmpDir, err := os.MkdirTemp("", "themis-update")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	binTmp, err := downloadVerifiedBinary(ctx, out, rel, binary, checksums, tmpDir, latestVer)
	if err != nil {
		return err
	}

	backupPath := exePath + ".backup"
	if backupErr := copyFile(exePath, backupPath); backupErr != nil {
		_, _ = fmt.Fprintf(out, "%s could not create backup of the current binary: %v\n", ui.LabelWarning.Render("warning"), backupErr)
	} else {
		_, _ = fmt.Fprintf(out, "%s backed up current binary to %s\n", ui.TextMuted.Render("i"), backupPath)
	}

	if replaceErr := replaceBinary(binTmp, exePath); replaceErr != nil {
		return fmt.Errorf("installing new binary: %w", replaceErr)
	}

	refreshInstalledCompletions(ctx, out, exePath)

	_, _ = fmt.Fprintf(out, "%s updated %s -> %s\n", ui.LabelSuccess.Render("✓"), currentVersion, latestVer)
	return nil
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Download and install the latest themis release",
		Example: "  themis system update        # check and apply latest stable release\n  themis system update --pre  # include pre-releases",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exePath, err := currentBinaryPath()
			if err != nil {
				return err
			}
			includePre, err := cmd.Flags().GetBool("pre")
			if err != nil {
				return err
			}
			return runUpdate(cmd.Context(), cmd.OutOrStdout(), exePath, buildinfo.GetVersionOnly(), includePre)
		},
	}
	cmd.Flags().Bool("pre", false, "include pre-releases in update check")
	return cmd
}

var updateCmd = newUpdateCmd()
