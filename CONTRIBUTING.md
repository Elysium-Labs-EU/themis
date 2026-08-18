# Contributing to themis

## Prerequisites

Go 1.26.6 or later and `make` are required. Verify with `go version` and `make --version`.

## Setup

```bash
git clone https://github.com/Elysium-Labs-EU/themis
cd themis
make setup
```

`make setup` installs the development toolchain (golangci-lint, nilaway, go-crap). Run `make help` to see all available targets; always prefer a make target over raw `go` or tool commands.

## Making Changes

Before touching any function or method, read [STYLE.md](STYLE.md) for the coding conventions that apply to all changes.

Open an issue before starting work on a non-trivial change. This avoids duplicate effort and makes sure the direction fits the project. Small fixes and documentation improvements can go straight to a PR.

Branch from `main` and name the branch after the change: `feat/osquery-source`, `fix/rollback-ordering`, `test/apply-idempotency`.

## Running Tests

```bash
make ci
```

This runs the full test, lint, nilaway, coverage-check, CRAP gate, and signing-key-sync check. It must pass before opening a PR. If lint reports violations, `make fix` resolves most of them automatically; run `make ci` again after.

Fixes mutate real host state (systemd, sysctl, package config), so unit tests alone don't exercise that path. If your change touches a fix, source, or the apply/rollback flow, also run `make test-integration-orb` on the OrbStack Debian VM (`make lynis-install-orb` once beforehand) — it runs as root and actually applies and rolls back changes. Don't rely on CI to catch a bad fix first.

## Commit Format

themis uses [Conventional Commits](https://www.conventionalcommits.org). The prefix determines which section of the changelog the commit appears in.

```
feat: add osquery drift source
fix: correct rollback ordering for chained fixes
test: cover apply idempotency for sysctl fixes
refactor: extract fix registry to its own package
docs: document THEMIS_INTEGRATION_MUTATE env variable
chore: bump golangci-lint to v2.11.0
```

Breaking changes go in the commit footer: `BREAKING CHANGE: <description>`.

## Opening a Pull Request

The PR description should explain *why* the change is needed, not just what it does. Link the issue it resolves with `Closes #N`.

All CI checks must be green. A PR that breaks `make ci` will not be reviewed until it is fixed.
