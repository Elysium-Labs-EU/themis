#!/usr/bin/env bash
# Fails if any tracked *.go file spells the empty interface as `interface{}`
# instead of the `any` alias (Go 1.18+). Cheap grep-based convention check;
# no golangci-lint rule enforces this today.
set -euo pipefail

matches="$(git grep -n -F --untracked -- 'interface{}' -- '*.go' || true)"

if [ -n "$matches" ]; then
  echo "check-any-convention: use 'any' instead of 'interface{}':" >&2
  echo "$matches" >&2
  exit 1
fi

echo "check-any-convention: no bare interface{} usages found"
