<p align="center">
  <img src=".github/logo.svg" alt="themis logo" width="120" height="120">
</p>

# themis - Automated Debian hardening CLI

[![GitHub](https://img.shields.io/badge/GitHub-themis-blue?logo=github)](https://github.com/Elysium-Labs-EU/themis)

Audits a Debian or Ubuntu host, fixes what it finds, and keeps it fixed. The workflow is check, plan, apply, rollback.

```bash
sudo themis check      # audit and list actionable findings
sudo themis plan       # show which fixes would be applied
sudo themis apply      # apply unsatisfied fixes, saving rollback state
sudo themis rollback   # undo the last apply
```

Findings come from pluggable sources. Lynis is one of them, not the whole tool. Sources detect; themis fixes.

## Features

* **Fixes, not just findings.** Every fix detects its own satisfied state and applies idempotently. Findings themis can act on lead the report; the rest print de-emphasized, `--all` shows them.
* **One-command rollback.** Every apply saves revert metadata, so a bad run undoes cleanly.
* **Pluggable audit sources.** Lynis, themis-native checks, osquery, and OpenSCAP register by name and merge into one report, grouped by who reported each finding.
* **Configurable.** A config file sets which sources run and their options. Scaffold it with `themis init`, change one value with `themis config set`.
* **Recurring scans.** `themis schedule enable` installs a systemd timer, launchd agent, or cron entry that runs a scan on an interval.
* **Drift detection.** `themis check` re-verifies fixes a prior apply satisfied and flags any that no longer hold, via osquery.
* **Machine-readable.** `themis api check` emits JSON for scripts and CI gates.
* **Single static binary.** Only Lynis is required for the default audit; osquery and OpenSCAP are optional per source.

If you have run Lynis and wished it fixed the findings instead of listing them, that is themis.

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

The default audit runs Lynis and themis-native checks, so [Lynis](https://cisofy.com/lynis/) must be on `PATH`. osquery and OpenSCAP are optional and only needed when their sources are enabled.

## Quick Start

```bash
sudo themis init       # scaffold a config file (optional)
sudo themis check      # audit
sudo themis apply      # fix
```

## Sources and fixes

Detecting and fixing stay separate.

Sources produce findings. Each source runs and reports; `themis check` merges them into one report grouped by source.

| Source | Reports | Requires |
|--------|---------|----------|
| `lynis` | Third-party system-hardening findings | Lynis on `PATH` (default) |
| `themis` | themis-native checks, things Lynis handles poorly or not at all | nothing |
| `osquery` | Drift on fixes a prior apply satisfied | osqueryi (optional) |
| `openscap` | CIS/DISA benchmark results from a SCAP datastream | oscap and content (optional) |

Fixes remediate findings. A fix maps to a finding's test ID, detects whether it is already satisfied, applies idempotently, and records rollback data. `themis plan` shows what would run, `themis apply` runs it. A source can surface an issue before any fix targets it, and a fix works no matter which source flagged it.

## Configuration

With no config file, the default audit runs Lynis, native checks, and osquery drift detection, with OpenSCAP off. A config file changes those defaults per host.

Config is read from `THEMIS_CONFIG` if set, else `/etc/themis/config.yaml` as root, else `~/.themis/config.yaml`. Precedence is built-in defaults, then the file, then CLI flags. A flag wins where it is passed. Omit any key to keep its default.

Scaffold a documented file:

```bash
sudo themis init         # prompts per source, writes a commented config.yaml
sudo themis init --yes   # writes defaults without prompting
```

Read and write single values for scripted setups:

```bash
themis config path                                   # print the resolved path
themis config get sources.lynis.enabled              # read one value
sudo themis config set sources.lynis.enabled false   # disable a source
```

The file:

```yaml
sources:
  lynis:
    enabled: true
    quick: false          # lynis --quick profile instead of a full audit
    skip_unchanged: false # reuse the last report when nothing lynis cares about changed
  native:
    enabled: true
  osquery:
    enabled: true         # drift detection, no-ops without osqueryi or prior apply state
  openscap:
    enabled: false
    content: ""           # path to a SCAP/XCCDF datastream, required when enabled
    profile: ""           # XCCDF profile ID, empty uses the datastream default
schedule:
  enabled: false
  interval: daily         # daily, weekly, or a raw systemd OnCalendar expression
  command: check          # check or apply, what each scheduled run invokes
```

To run native checks only, disable `lynis`, `osquery`, and `openscap`. To add OpenSCAP, set `openscap.enabled: true` and point `openscap.content` at a datastream.

## Recurring scans

There is no daemon. A recurring scan is an OS-native unit that runs a one-shot themis command.

```bash
sudo themis schedule enable                      # install from the config schedule block
sudo themis schedule enable --interval weekly --command apply
sudo themis schedule status                      # installed state
sudo themis schedule disable                     # remove it
```

The backend is chosen per host: a systemd timer, a launchd agent on macOS, or a cron entry. `interval` takes `daily`, `weekly`, or a systemd `OnCalendar` expression. `command` is `check` or `apply`.

## Drift detection

Config a fix touched can drift back out of compliance between apply runs, someone edits it back, a package resets it, a service stops. `themis check` re-verifies every fix a prior apply confirmed, independently, via [osquery](https://osquery.io/) system tables. A fix that no longer holds is reported as drift, in its own section ahead of fresh findings, and under `"drift"` in `themis api check`.

Drift detection is optional and self-skipping. With no osqueryi and no prior apply state at `/var/lib/themis/state.json`, `themis check` runs as before, no error, no drift section. Covered fixes and the query mapping live in `internal/osquery/checks.go`.

## Release integrity

`install.sh` and `themis system update` download only from `github.com` over HTTPS, verify the binary sha256 against the release manifest, and verify an ECDSA signature over it. A release with no signature is warned about, not rejected. See [SECURITY.md](SECURITY.md) for the full verified-install steps.

## Commands

| Command | Description |
|---------|-------------|
| `themis init` | Scaffold a commented `config.yaml`, interactive or `--yes` |
| `themis check` | Run an audit and list actionable findings, `--all` shows the rest |
| `themis plan` | Show which fixes would be applied |
| `themis apply` | Apply unsatisfied fixes and save rollback state |
| `themis rollback` | Revert the fixes from the last apply |
| `themis config path\|get\|set` | Read or write single config values |
| `themis schedule enable\|disable\|status` | Manage a recurring scan |
| `themis api check` | Return findings merged with fixes as JSON |
| `themis system version\|update\|uninstall` | Manage the themis binary |
| `themis completion` | Install or print shell tab completion |

## License

Apache License 2.0 - see [LICENSE](LICENSE).
