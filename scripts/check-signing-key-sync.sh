#!/usr/bin/env bash
# Fails if install.sh's embedded RELEASE_SIGNING_PUBKEY ever drifts from
# cmd/update.go's embedded releaseSigningPublicKeyPEM. The two are meant to be
# byte-identical (see the "Keep in sync" comments on both), but nothing
# enforced that until now -- a key rotation (240476b) updated only
# cmd/update.go and left install.sh checking releases against a retired key.
# This is the second time this drift class has happened (same bug as eos#38).
#
# Usage: check-signing-key-sync.sh [install.sh path] [update.go path]
set -euo pipefail

INSTALL_SH="${1:-install.sh}"
UPDATE_GO="${2:-cmd/update.go}"

extract_pem() {
  # Print the BEGIN..END block, then strip whatever surrounds the markers on
  # their own lines (install.sh's bash quoting, update.go's Go raw string)
  # so only the PEM itself is left to compare.
  sed -n '/-----BEGIN PUBLIC KEY-----/,/-----END PUBLIC KEY-----/p' "$1" |
    sed -e 's/.*\(-----BEGIN PUBLIC KEY-----\)/\1/' -e 's/\(-----END PUBLIC KEY-----\).*/\1/' |
    sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

install_key="$(extract_pem "$INSTALL_SH")"
update_key="$(extract_pem "$UPDATE_GO")"

if [ -z "$install_key" ]; then
  echo "check-signing-key-sync: no PEM public key found in $INSTALL_SH" >&2
  exit 1
fi
if [ -z "$update_key" ]; then
  echo "check-signing-key-sync: no PEM public key found in $UPDATE_GO" >&2
  exit 1
fi

if [ "$install_key" != "$update_key" ]; then
  echo "check-signing-key-sync: release signing public key in $INSTALL_SH does not match $UPDATE_GO" >&2
  echo "  $INSTALL_SH embeds:" >&2
  echo "$install_key" | sed 's/^/    /' >&2
  echo "  $UPDATE_GO embeds:" >&2
  echo "$update_key" | sed 's/^/    /' >&2
  exit 1
fi

echo "check-signing-key-sync: $INSTALL_SH and $UPDATE_GO embed the same release signing public key"
