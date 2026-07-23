package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const scriptPath = "check-signing-key-sync.sh"

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

// runScript invokes check-signing-key-sync.sh against installPath/updatePath
// and returns its combined output and exit error (nil on success).
func runScript(t *testing.T, installPath, updatePath string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", scriptPath, installPath, updatePath)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCheckSigningKeySyncPassesOnRealRepoFiles(t *testing.T) {
	// The actual regression this task fixes: install.sh's embedded key must
	// match cmd/update.go's after the 240476b key rotation.
	out, err := runScript(t, filepath.Join("..", "install.sh"), filepath.Join("..", "cmd", "update.go"))
	if err != nil {
		t.Fatalf("expected install.sh and cmd/update.go to be in sync, got error: %v\noutput:\n%s", err, out)
	}
}

func TestCheckSigningKeySyncDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	install := writeFixture(t, dir, "install.sh", `readonly RELEASE_SIGNING_PUBKEY='-----BEGIN PUBLIC KEY-----
DEADBEEFxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx==
-----END PUBLIC KEY-----'
`)
	update := writeFixture(t, dir, "update.go", "const releaseSigningPublicKeyPEM = `-----BEGIN PUBLIC KEY-----\n"+
		"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZo6eWxjF1xhHMI/MyUNptSdkxuHM\n"+
		"qAeiDXd1PrPNR3I1N1radAb1df3CPt0WjZQmuTesJLQiDL91WwVt7fraSA==\n"+
		"-----END PUBLIC KEY-----\n`\n")

	out, err := runScript(t, install, update)
	if err == nil {
		t.Fatalf("expected mismatched keys to fail, got success\noutput:\n%s", out)
	}
	if want := "does not match"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the mismatch (%q), got:\n%s", want, out)
	}
}

func TestCheckSigningKeySyncPassesOnIdenticalKeys(t *testing.T) {
	dir := t.TempDir()
	pem := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZo6eWxjF1xhHMI/MyUNptSdkxuHM
qAeiDXd1PrPNR3I1N1radAb1df3CPt0WjZQmuTesJLQiDL91WwVt7fraSA==
-----END PUBLIC KEY-----`
	install := writeFixture(t, dir, "install.sh", "readonly RELEASE_SIGNING_PUBKEY='"+pem+"'\n")
	update := writeFixture(t, dir, "update.go", "const releaseSigningPublicKeyPEM = `"+pem+"\n`\n")

	out, err := runScript(t, install, update)
	if err != nil {
		t.Fatalf("expected identical keys to pass, got error: %v\noutput:\n%s", err, out)
	}
}

func TestCheckSigningKeySyncFailsOnMissingPEM(t *testing.T) {
	dir := t.TempDir()
	install := writeFixture(t, dir, "install.sh", "readonly RELEASE_SIGNING_PUBKEY='no key here'\n")
	update := writeFixture(t, dir, "update.go", "const releaseSigningPublicKeyPEM = `-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----\n`\n")

	out, err := runScript(t, install, update)
	if err == nil {
		t.Fatalf("expected missing PEM in install.sh to fail, got success\noutput:\n%s", out)
	}
	if want := "no PEM public key found"; !strings.Contains(out, want) {
		t.Errorf("expected output to explain the missing key (%q), got:\n%s", want, out)
	}
}
