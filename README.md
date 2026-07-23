<p align="center">
  <img src=".github/logo.svg" alt="themis logo" width="120" height="120">
</p>

# themis - Automated Debian hardening CLI

[![GitHub](https://img.shields.io/badge/GitHub-themis-blue?logo=github)](https://github.com/Elysium-Labs-EU/themis)

themis merges findings from pluggable audit sources ([Lynis](https://cisofy.com/lynis/), plus themis-native checks) with a check/plan/apply/rollback workflow. It maps flagged findings to concrete fixes and applies them idempotently with rollback metadata.

## Features

* **Actionable findings only** by default, findings with no themis fix and no solution hint print de-emphasized instead of a full table row; `--all` promotes them back.
* **Idempotent fixes**, each registered fix knows how to detect its own satisfied state before applying anything.
* **Rollback metadata** saved automatically on every `apply`, so a bad hardening run can be undone with one command.
* **Machine-readable output** via `themis api check`, for scripting or CI gates.
* **Zero runtime dependencies** beyond Lynis itself, single static binary.

## Install

**curl**
```bash
curl -sSL https://raw.githubusercontent.com/Elysium-Labs-EU/themis/main/install.sh -o install.sh
sudo bash install.sh
```

**wget**
```bash
wget https://raw.githubusercontent.com/Elysium-Labs-EU/themis/main/install.sh
sudo bash install.sh
```

**From source**
```bash
git clone https://github.com/Elysium-Labs-EU/themis
cd themis
go build -o themis
```

Requires [Lynis](https://cisofy.com/lynis/) on PATH; themis shells out to it for the audit.

### Release integrity

`install.sh` and `themis system update` only download from `github.com` over HTTPS, verify the downloaded binary's sha256 against the release's `sha256sums.txt`, and — once a release publishes one — verify an ECDSA P-256 signature over `sha256sums.txt` (`sha256sums.txt.sig`) against a public key embedded in both `install.sh` and the binary. A release with no signature is only warned about, not rejected; see `requireReleaseSignature` in `cmd/update.go`.

That covers the *binary* themis downloads for you — but the quick-install one-liners above fetch `install.sh` itself from `main`, a mutable branch with no integrity check on the script text before it's piped to `bash`.

### Verified installation

If you're installing on a machine you care about, use this path instead of the quick-install one-liners: `install.sh` is included in every release's signed `sha256sums.txt`, so fetching it from the release (not `main`) lets you verify it the same way as the binary, before running it.

```bash
VERSION=v0.0.3-rc.1   # pin to the release you intend to install -- see: https://github.com/Elysium-Labs-EU/themis/releases
REPO=Elysium-Labs-EU/themis

curl -sSL -o install.sh        "https://github.com/${REPO}/releases/download/${VERSION}/install.sh"
curl -sSL -o sha256sums.txt     "https://github.com/${REPO}/releases/download/${VERSION}/sha256sums.txt"
curl -sSL -o sha256sums.txt.sig "https://github.com/${REPO}/releases/download/${VERSION}/sha256sums.txt.sig"

cat > release-signing-pubkey.pem <<'EOF'
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZo6eWxjF1xhHMI/MyUNptSdkxuHM
qAeiDXd1PrPNR3I1N1radAb1df3CPt0WjZQmuTesJLQiDL91WwVt7fraSA==
-----END PUBLIC KEY-----
EOF

# 1. sha256sums.txt itself is genuinely from us
openssl dgst -sha256 -verify release-signing-pubkey.pem -signature sha256sums.txt.sig sha256sums.txt
# 2. install.sh matches what that manifest attests to
sha256sum -c <(grep install.sh sha256sums.txt)

sudo bash install.sh
```

The public key above must match `RELEASE_SIGNING_PUBKEY` in `install.sh` and `releaseSigningPublicKeyPEM` in `cmd/update.go` exactly — CI's `check-signing-key-sync.sh` gates on that, but verifying against the copy in this README (fetched independently of the release itself) is what actually roots the trust chain in something other than the artifact you're checking.

## Quick Start

```bash
# Run an audit and list actionable findings
sudo themis check

# Show which registered fixes would be applied
sudo themis plan

# Apply all unsatisfied fixes, saving rollback state
sudo themis apply

# Undo the fixes from the last apply
sudo themis rollback
```

## Commands

| Command | Description |
|---------|-------------|
| `themis check` | Run an audit and list actionable findings |
| `themis check --all` | Also show findings with no themis fix and no solution hint |
| `themis plan` | Show which registered fixes would be applied |
| `themis apply` | Apply all unsatisfied registered fixes and save rollback state |
| `themis rollback` | Revert the fixes applied by the last `apply` |
| `themis api check` | Return audit findings merged with themis fixes as JSON |
| `themis system version` | Print version, git commit, and build date |
| `themis system update` | Check for and install the latest themis release |
| `themis system uninstall` | Remove the themis binary |
| `themis completion` | Detect your shell and interactively install tab completion |
| `themis completion bash\|zsh\|fish` | Print the completion script for a shell to stdout |

## License

Apache License 2.0 - see [LICENSE](LICENSE).
