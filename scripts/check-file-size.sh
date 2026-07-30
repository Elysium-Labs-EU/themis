#!/usr/bin/env bash
# Fails a PR diff that adds or grows a file past a size threshold, or that
# introduces a binary blob -- unless that path is tracked through Git LFS
# (a .gitattributes "filter=lfs" pattern). themis doesn't use LFS today, so
# in practice this blocks any large or binary file from landing in history
# by accident (e.g. a debug binary or a pasted-in fixture dump); a legitimate
# large asset should be added to .gitattributes as LFS-tracked first.
#
#   CHECK_FILE_SIZE_BASE   base ref (default: origin/main); CI sets it to PR target
#   CHECK_FILE_SIZE_MAX    max file size in bytes (default: 1048576, 1 MiB)
set -euo pipefail

BASE_REF="${CHECK_FILE_SIZE_BASE:-origin/main}"
MAX_BYTES="${CHECK_FILE_SIZE_MAX:-1048576}"

# CI checkouts are often shallow; make the base ref resolvable.
if ! git rev-parse --verify --quiet "$BASE_REF" >/dev/null 2>&1; then
  git fetch --quiet origin "${BASE_REF#origin/}" 2>/dev/null || true
fi
if git rev-parse --verify --quiet "$BASE_REF" >/dev/null 2>&1; then
  MERGE_BASE="$(git merge-base "$BASE_REF" HEAD 2>/dev/null || echo "$BASE_REF")"
else
  echo "check-file-size: base ref '$BASE_REF' unresolvable; nothing to compare against, passing." >&2
  exit 0
fi

fail=0
while IFS= read -r -d '' path; do
  [ -f "$path" ] || continue # skip deleted files

  if git check-attr filter -- "$path" | grep -q 'filter: lfs$'; then
    continue # LFS-tracked, exempt by design
  fi

  size="$(wc -c <"$path" | tr -d ' ')"
  if [ "$size" -gt "$MAX_BYTES" ]; then
    echo "check-file-size: $path is ${size} bytes, exceeds ${MAX_BYTES} byte limit (track via Git LFS if intentional)" >&2
    fail=1
  fi

  if ! git diff --numstat "$MERGE_BASE" -- "$path" | grep -q '^[0-9]'; then
    echo "check-file-size: $path is a binary blob (track via Git LFS if intentional)" >&2
    fail=1
  fi
done < <(git diff --name-only --diff-filter=ACMR -z "$MERGE_BASE" -- .)

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "check-file-size: no oversized or unexpected binary files in diff against $BASE_REF"
