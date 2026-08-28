#!/usr/bin/env bash
# Fails if golangci-lint's pinned version ever drifts across its four
# consumers (Makefile, lefthook.yml, release.yml, .golangci-lint-version
# itself). All four are meant to resolve the same version from
# .golangci-lint-version at invocation time rather than hardcoding a
# string, so a bump stays a one-line change to that file.
#
# Usage: check-golangci-pin.sh [repo root]
set -euo pipefail

ROOT="${1:-.}"
VERSION_FILE="$ROOT/.golangci-lint-version"
MAKEFILE="$ROOT/Makefile"
LEFTHOOK="$ROOT/lefthook.yml"

fail=0

if [ ! -f "$VERSION_FILE" ]; then
  echo "check-golangci-pin: $VERSION_FILE is missing" >&2
  exit 1
fi

version="$(tr -d '[:space:]' <"$VERSION_FILE")"
if ! [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "check-golangci-pin: $VERSION_FILE must contain an exact vMAJOR.MINOR.PATCH version, got '$version'" >&2
  fail=1
fi

if [ -f "$MAKEFILE" ]; then
  if grep -qE '^[[:space:]]*golangci-lint[[:space:]]' "$MAKEFILE"; then
    echo "check-golangci-pin: $MAKEFILE invokes bare golangci-lint instead of the pinned 'go run .../golangci-lint@\$(GOLANGCI_LINT_VERSION)' form" >&2
    fail=1
  fi
  if ! grep -q 'golangci-lint/v2/cmd/golangci-lint@\$(GOLANGCI_LINT_VERSION)' "$MAKEFILE"; then
    echo "check-golangci-pin: $MAKEFILE has no pinned 'go run .../golangci-lint@\$(GOLANGCI_LINT_VERSION)' invocation" >&2
    fail=1
  fi
else
  echo "check-golangci-pin: $MAKEFILE is missing" >&2
  fail=1
fi

if [ -f "$LEFTHOOK" ]; then
  if grep -qE 'golangci-lint(/v2/cmd/golangci-lint)?@v[0-9]' "$LEFTHOOK"; then
    echo "check-golangci-pin: $LEFTHOOK hardcodes a golangci-lint version instead of reading .golangci-lint-version" >&2
    fail=1
  fi
  if grep -qE 'golangci-lint (fmt|run)' "$LEFTHOOK"; then
    echo "check-golangci-pin: $LEFTHOOK invokes bare golangci-lint instead of the pinned go run form" >&2
    fail=1
  fi
else
  echo "check-golangci-pin: $LEFTHOOK is missing" >&2
  fail=1
fi

if [ -d "$ROOT/.github/workflows" ]; then
  while IFS= read -r workflow; do
    if grep -qE 'version:[[:space:]]*v?[0-9]+\.[0-9]+' "$workflow"; then
      echo "check-golangci-pin: $workflow hardcodes a golangci-lint-action version instead of reading .golangci-lint-version" >&2
      fail=1
    fi
  done < <(grep -lr 'golangci-lint-action' "$ROOT/.github/workflows" 2>/dev/null || true)
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "check-golangci-pin: golangci-lint pin is consistent at $version across Makefile, lefthook.yml, and workflows"
