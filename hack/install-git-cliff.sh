#!/usr/bin/env bash
# Download a pinned git-cliff release binary into bin/git-cliff.
# Usage: hack/install-git-cliff.sh <version> <output-path>
# Example: hack/install-git-cliff.sh v2.13.1 bin/git-cliff
set -euo pipefail

VERSION="${1:?version required (e.g. v2.13.1)}"
OUT="${2:?output path required (e.g. bin/git-cliff)}"
VER="${VERSION#v}"

case "$(uname -m)" in
  x86_64) arch=x86_64 ;;
  aarch64 | arm64) arch=aarch64 ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

case "$(uname -s)" in
  Linux) platform="${arch}-unknown-linux-gnu" ;;
  Darwin) platform="${arch}-apple-darwin" ;;
  *)
    echo "unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

asset="git-cliff-${VER}-${platform}.tar.gz"
url="https://github.com/orhun/git-cliff/releases/download/${VERSION}/${asset}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/fetch.sh
source "${root}/hack/lib/fetch.sh"

mkdir -p "$(dirname "${root}/${OUT}")"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# Upstream publishes one `<sha512>  <asset>` file per asset and no SHA256, so this compares with
# sha512sum rather than verify_sha256. The digest is in hand before the tarball is fetched, and a
# failed checksum fetch stops here: nothing is ever installed unverified.
fetch_to "${url}.sha512" "${tmpdir}/${asset}.sha512" "git-cliff ${VERSION} checksum"
expected="$(awk -v file="${asset}" '$2 == file || $2 == "*" file {print $1}' "${tmpdir}/${asset}.sha512")"
if [[ ! "${expected}" =~ ^[0-9a-f]{128}$ ]]; then
  echo "failed to resolve a sha512 for ${asset} from ${url}.sha512" >&2
  exit 1
fi
fetch_to "${url}" "${tmpdir}/${asset}" "git-cliff ${VERSION} tarball"
actual="$(sha512sum "${tmpdir}/${asset}" | awk '{print $1}')"
if [[ "${actual}" != "${expected}" ]]; then
  echo "checksum mismatch for ${asset}" >&2
  echo "  expected: ${expected}" >&2
  echo "  actual:   ${actual}" >&2
  exit 1
fi
tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}"
install -m 0755 "${tmpdir}/git-cliff-${VER}/git-cliff" "${root}/${OUT}"
echo "installed ${OUT} (${VERSION} ${platform})"
