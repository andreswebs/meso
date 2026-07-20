#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

: "${TAG:?TAG is required}"
: "${REPO:?REPO is required}"
: "${SBOM:?SBOM is required}"
DIST_DIR="${DIST_DIR:-dist}"

prerelease=()
# Pre-release tags carry a suffix after a hyphen (e.g. v1.2.0-rc1).
case "${TAG}" in
*-*) prerelease=(--prerelease) ;;
esac

# The notes template stays a literal heredoc so its backtick code fences are not
# treated as command substitution; @REPO@ and @SBOM@ are substituted afterwards.
notes="$(
    cat <<'EOF'
The Go module proxy serves this release's source with sum.golang.org
integrity; the artifacts below are verifiable release metadata: the
module's SBOM, checksums over it, Sigstore keyless signatures, and SLSA
build provenance. Verification needs cosign v3+ and gh v2.49+.

## Verifying this release

### Checksums signature (cosign)

```sh
cosign verify-blob \
  --bundle SHA256SUMS.txt.sigstore.json \
  --certificate-identity-regexp "https://github.com/@REPO@/.github/workflows/release.yml@refs/tags/.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  SHA256SUMS.txt
sha256sum -c SHA256SUMS.txt
```

### Build provenance (GitHub attestations)

```sh
gh attestation verify @SBOM@ --repo @REPO@
```

### SBOM signature (cosign)

```sh
cosign verify-blob \
  --bundle @SBOM@.sigstore.json \
  --certificate-identity-regexp "https://github.com/@REPO@/.github/workflows/release.yml@refs/tags/.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  @SBOM@
```
EOF
)"
notes="${notes//@REPO@/${REPO}}"
notes="${notes//@SBOM@/${SBOM}}"

gh release create "${TAG}" \
    --title "${TAG}" \
    --notes "${notes}" \
    --generate-notes \
    "${prerelease[@]}" \
    "${DIST_DIR}/SHA256SUMS.txt" \
    "${DIST_DIR}/SHA256SUMS.txt.sigstore.json" \
    "${DIST_DIR}/${SBOM}" \
    "${DIST_DIR}/${SBOM}.sigstore.json"
