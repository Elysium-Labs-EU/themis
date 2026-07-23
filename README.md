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

### Release integrity

`install.sh` and `themis system update` only download from `github.com` over HTTPS, verify the downloaded binary's sha256 against the release's `sha256sums.txt`, and — once a release publishes one — verify an ECDSA P-256 signature over `sha256sums.txt` (`sha256sums.txt.sig`) against a public key embedded in both `install.sh` and the binary. A release with no signature is only warned about, not rejected; see `requireReleaseSignature` in `cmd/update.go`.

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
