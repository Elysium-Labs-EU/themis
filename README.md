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
* **Drift detection** via [osquery](https://osquery.io/) (optional), `themis check` flags fixes that were satisfied by a prior `apply` but no longer hold, surfaced separately from fresh findings — see [Drift detection](#drift-detection) below.
* **Machine-readable output** via `themis api check`, for scripting or CI gates.
* **Zero required runtime dependencies** beyond Lynis itself, single static binary; osquery is optional and only used for drift detection.

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

Requires [Lynis](https://cisofy.com/lynis/) on PATH; themis shells out to it for the audit. [osquery](https://osquery.io/) is optional and only needed for drift detection — see [Drift detection](#drift-detection).

### OpenSCAP (optional)

themis can additionally run [OpenSCAP](https://www.open-scap.org/) (`oscap xccdf eval`) against CIS/DISA benchmark content, e.g. the [SCAP Security Guide](https://github.com/ComplianceAsCode/content):

```bash
sudo apt install libopenscap8 scap-security-guide-debian
sudo themis check --scap-content /usr/share/xml/scap/ssg/content/ssg-debian12-ds.xml
sudo themis check --scap-content /usr/share/xml/scap/ssg/content/ssg-debian12-ds.xml --scap-profile xccdf_org.ssgproject.content_profile_cis_level1_server
```

`--scap-content` is required to enable the OpenSCAP source; without it themis runs Lynis and its native checks only, unchanged from before. `--scap-profile` scopes the scan to one XCCDF profile (default: the datastream's own default profile). OpenSCAP rule IDs (`xccdf_org.ssgproject.content_rule_...`) are a separate namespace from Lynis test IDs, so they never collide in `fix.Registry`; findings surface for review even before a themis fix targets them.

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

## Drift detection

Between `themis apply` runs, config a fix touched (an sshd directive, a sysctl, a service) can drift back out of compliance — someone edits it back, a package reinstall resets it, a service gets disabled. `themis check` re-verifies every fix a *prior* `apply` confirmed satisfied, independently of the same detection logic `apply` used, via [osquery](https://osquery.io/)'s system tables (`sshd_config`, `system_controls`, `systemd_units`). A fix that no longer holds is reported as **drift**, printed in its own section ahead of the regular findings (and under `"drift"` in `themis api check`'s JSON) rather than mixed in with fresh suggestions — a regression on something already fixed once is a different signal than something never addressed.

**Prerequisites**

* Install `osqueryi` (part of the [osquery](https://osquery.io/downloads/) package) and make sure it resolves from `/usr/sbin`, `/usr/bin`, `/sbin`, `/bin`, `/usr/local/sbin`, or `/usr/local/bin` — themis never resolves external commands through `$PATH`.
* No osquery config file is required; themis invokes `osqueryi --json "<query>"` directly per check, it does not run `osqueryd` or use osquery's config/flag files.
* Drift detection is entirely optional and self-skipping: with no `osqueryi` binary installed, or no prior `themis apply` state (`/var/lib/themis/state.json`) on the host, `themis check` runs exactly as before with no error and no drift section.

Currently covered: `SSH-7408-ROOTLOGIN`, `SSH-7408-PASSWDAUTH`, `KRNL-6000`, `THEMIS-FAIL2BAN` (see `internal/osquery/checks.go` for the query-to-fix mapping). `FIRE-4590` and `PKGS-7392` aren't covered — see the doc comment on `osquery.Checks` for why.

## Commands

| Command | Description |
|---------|-------------|
| `themis init` | Interactively scaffold a commented `config.yaml` (`--yes` writes defaults) |
| `themis check` | Run an audit and list actionable findings |
| `themis check --all` | Also show findings with no themis fix and no solution hint |
| `themis check --scap-content <path>` | Also run OpenSCAP against the given SCAP/XCCDF datastream |
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
