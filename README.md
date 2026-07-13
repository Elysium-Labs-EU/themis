# themis - Lynis-driven Debian hardening CLI

[![Codeberg](https://img.shields.io/badge/Codeberg-themis-blue?logo=codeberg)](https://codeberg.org/Elysium_Labs/themis)

themis wraps [Lynis](https://cisofy.com/lynis/)'s audit findings with a check/plan/apply/rollback workflow. It reads Lynis's report, maps flagged findings to concrete fixes, and applies them idempotently with rollback metadata.

## Features

* **Actionable findings only** by default, findings with no themis fix and no Lynis solution hint print de-emphasized instead of a full table row; `--all` promotes them back.
* **Idempotent fixes**, each registered fix knows how to detect its own satisfied state before applying anything.
* **Rollback metadata** saved automatically on every `apply`, so a bad hardening run can be undone with one command.
* **Machine-readable output** via `themis api check`, for scripting or CI gates.
* **Zero runtime dependencies** beyond Lynis itself, single static binary.

## Install

**curl**
```bash
curl -sSL https://codeberg.org/Elysium_Labs/themis/raw/branch/main/install.sh -o install.sh
sudo bash install.sh
```

**wget**
```bash
wget https://codeberg.org/Elysium_Labs/themis/raw/branch/main/install.sh
sudo bash install.sh
```

**From source**
```bash
git clone https://codeberg.org/Elysium_Labs/themis
cd themis
go build -o themis
```

Requires [Lynis](https://cisofy.com/lynis/) on PATH; themis shells out to it for the audit.

## Quick Start

```bash
# Run a Lynis audit and list actionable findings
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
| `themis check` | Run a Lynis audit and list actionable findings |
| `themis check --all` | Also show findings with no themis fix and no Lynis solution hint |
| `themis plan` | Show which registered fixes would be applied |
| `themis apply` | Apply all unsatisfied registered fixes and save rollback state |
| `themis rollback` | Revert the fixes applied by the last `apply` |
| `themis api check` | Return Lynis findings merged with themis fixes as JSON |
| `themis version` | Print version, git commit, and build date |

## License

License not yet finalized for this repository.
