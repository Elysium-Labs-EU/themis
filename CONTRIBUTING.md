# Contributing to themis

## Prerequisites

Go 1.26.5 or later and `make` are required. Verify with `go version` and `make --version`.

## Setup

```bash
git clone https://github.com/Elysium-Labs-EU/themis
cd themis
make setup
```

`make setup` installs the development toolchain (golangci-lint, nilaway, go-crap - same versions as eos). Run `make help` to see all available targets; always prefer a make target over raw `go` or tool commands.

## Making Changes

Before touching any function or method, read [STYLE.md](STYLE.md) for the coding conventions that apply to all changes.

Open an issue before starting work on a non-trivial change. This avoids duplicate effort and makes sure the direction fits the project. Small fixes and documentation improvements can go straight to a PR.

Branch from `main` and name the branch after the change: `feat/audit-source-plugin`, `fix/report-parse-error`, `test/lynis-integration`.

## Running Tests

```bash
make ci
```

This runs the full test, lint, sg, nilaway, coverage, and CRAP gate. It must pass before opening a PR. If lint reports violations, `make fix` resolves most of them automatically; run `make ci` again after.

If your change touches OS-facing audit source code, also run `make test-integration` (or `make test-integration-orb` for the OrbStack-backed variant) before opening a PR - some behavior only surfaces against a real target system and won't fail in the unit suite.

## Commit Format

themis uses [Conventional Commits](https://www.conventionalcommits.org). The prefix determines which section of the changelog the commit appears in.

```
feat: add lynis audit source plugin
fix: correct CVE severity parsing
test: cover report aggregation under partial source failure
refactor: extract source registry to pure func
docs: document THEMIS_CONFIG_PATH env variable
chore: bump golangci-lint to v2.11.0
```

Breaking changes go in the commit footer: `BREAKING CHANGE: <description>`.

## Opening a Pull Request

Fill in the PR template. The summary should explain *why* the change is needed, not just what it does. Link the issue it resolves with `Closes #N`.

All CI checks must be green. A PR that breaks `make ci` will not be reviewed until it is fixed.
