# Security

## Release integrity

`install.sh` and `themis system update` download only from `github.com` over HTTPS, verify the binary sha256 against the release `sha256sums.txt`, and verify an ECDSA P-256 signature over it against a public key embedded in both `install.sh` and the binary. A release with no signature is warned about, not rejected. See `requireReleaseSignature` in `cmd/update.go`.

## Verified installation

For a machine you care about, verify `install.sh` against the release signed manifest before running it, instead of piping from `main`:

```bash
VERSION=v0.0.3   # pin to a release: https://github.com/Elysium-Labs-EU/themis/releases
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

openssl dgst -sha256 -verify release-signing-pubkey.pem -signature sha256sums.txt.sig sha256sums.txt
sha256sum -c <(grep install.sh sha256sums.txt)

sudo bash install.sh
```

The public key must match `RELEASE_SIGNING_PUBKEY` in `install.sh` and `releaseSigningPublicKeyPEM` in `cmd/update.go`. CI's `check-signing-key-sync.sh` gates on that.
