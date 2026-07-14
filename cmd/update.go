package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"codeberg.org/Elysium_Labs/themis/internal/buildinfo"
	"codeberg.org/Elysium_Labs/themis/internal/release"
	"codeberg.org/Elysium_Labs/themis/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const themisRepo = "Elysium_Labs/themis"

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

// runUpdate implements `themis update` against an explicit exePath, so it
// can be exercised in tests without touching the test binary itself
// (os.Executable() under `go test` is the test binary).
func runUpdate(ctx context.Context, out io.Writer, exePath, currentVersion string) error {
	var rel release.Release
	err := ui.WithSpinner("Checking for updates...", func() error {
		var err error
		rel, err = release.FetchLatest(ctx, themisRepo)
		return err
	})
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	currentVer, latestVer := normalizeSemver(currentVersion), rel.TagName
	if semver.IsValid(currentVer) && semver.IsValid(latestVer) && semver.Compare(currentVer, latestVer) >= 0 {
		_, _ = fmt.Fprintf(out, "%s already on the latest version (%s)\n", ui.LabelSuccess.Render("✓"), currentVersion)
		return nil
	}

	_, _ = fmt.Fprintf(out, "%s new version available: %s -> %s\n", ui.LabelInfo.Render("i"), currentVersion, latestVer)

	arch, err := hostArch()
	if err != nil {
		return err
	}
	asset, ok := rel.AssetFor(arch)
	if !ok {
		return &ui.UserError{Err: fmt.Errorf("release %s has no asset for linux-%s", latestVer, arch)}
	}
	checksums, ok := rel.ChecksumsAsset()
	if !ok {
		return &ui.UserError{Err: fmt.Errorf("release %s is missing sha256sums.txt", latestVer)}
	}

	if writeErr := release.CheckWritable(filepath.Dir(exePath)); writeErr != nil {
		return &ui.UserError{
			Err:  fmt.Errorf("%s is not writable: %w", filepath.Dir(exePath), writeErr),
			Hint: "sudo themis update",
		}
	}

	tmpDir, err := os.MkdirTemp("", "themis-update")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	binTmp := filepath.Join(tmpDir, "themis")
	err = ui.WithSpinner(fmt.Sprintf("Downloading %s...", latestVer), func() error {
		return release.Download(ctx, asset.DownloadURL, binTmp)
	})
	if err != nil {
		return fmt.Errorf("downloading update: %w", err)
	}

	checksumsTmp := filepath.Join(tmpDir, "sha256sums.txt")
	if dlErr := release.Download(ctx, checksums.DownloadURL, checksumsTmp); dlErr != nil {
		return fmt.Errorf("downloading checksums: %w", dlErr)
	}
	checksumsData, err := os.ReadFile(checksumsTmp) //nolint:gosec // fixed name in a themis-owned temp dir
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}
	if verifyErr := release.VerifyChecksum(binTmp, string(checksumsData), asset.Name); verifyErr != nil {
		return &ui.UserError{Err: verifyErr}
	}
	_, _ = fmt.Fprintf(out, "%s checksum verified\n", ui.LabelSuccess.Render("✓"))

	backupPath := exePath + ".backup"
	if backupErr := release.CopyFile(exePath, backupPath); backupErr != nil {
		_, _ = fmt.Fprintf(out, "%s could not create backup of the current binary: %v\n", ui.LabelWarning.Render("warning"), backupErr)
	} else {
		_, _ = fmt.Fprintf(out, "%s backed up current binary to %s\n", ui.TextMuted.Render("i"), backupPath)
	}

	if replaceErr := release.ReplaceBinary(binTmp, exePath); replaceErr != nil {
		return fmt.Errorf("installing new binary: %w", replaceErr)
	}

	_, _ = fmt.Fprintf(out, "%s updated %s -> %s\n", ui.LabelSuccess.Render("✓"), currentVersion, latestVer)
	return nil
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Download and install the latest themis release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exePath, err := currentBinaryPath()
			if err != nil {
				return err
			}
			return runUpdate(cmd.Context(), cmd.OutOrStdout(), exePath, buildinfo.GetVersionOnly())
		},
	}
	return cmd
}

var updateCmd = newUpdateCmd()
