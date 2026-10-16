#!/usr/bin/env bash
# Download a pinned helm-docs release binary into bin/helm-docs.
# Usage: hack/install-helm-docs.sh <version> <output-path>
# Example: hack/install-helm-docs.sh v1.14.2 bin/helm-docs
set -euo pipefail

VERSION="${1:?version required (e.g. v1.14.2)}"
OUT="${2:?output path required (e.g. bin/helm-docs)}"
VER="${VERSION#v}"

case "$(uname -m)" in
  x86_64) arch=x86_64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

case "$(uname -s)" in
  Linux) os=Linux ;;
  Darwin) os=Darwin ;;
  *)
    echo "unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

asset="helm-docs_${VER}_${os}_${arch}.tar.gz"
base_url="https://github.com/norwoodj/helm-docs/releases/download/${VERSION}"
url="${base_url}/${asset}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/verify-sha256.sh
source "${root}/hack/lib/verify-sha256.sh"
# shellcheck source=lib/fetch.sh
source "${root}/hack/lib/fetch.sh"

mkdir -p "$(dirname "${root}/${OUT}")"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# The digest comes from the release's checksums.txt (`<sha256>  <asset>` per asset) and is in hand
# before the tarball is fetched; a failed checksum fetch stops here. Like git-cliff's, it catches a
# corrupted or swapped download, not a compromised release.
fetch_to "${base_url}/checksums.txt" "${tmpdir}/checksums.txt" "helm-docs ${VERSION} checksums"
expected="$(awk -v file="${asset}" '$2 == file {print $1}' "${tmpdir}/checksums.txt")"
# One full SHA256 or nothing: a missing, short or doubled line is refused before any comparison.
if [[ ! "${expected}" =~ ^[0-9a-f]{64}$ ]]; then
  echo "failed to resolve a checksum for ${asset} from ${base_url}/checksums.txt" >&2
  exit 1
fi
fetch_to "${url}" "${tmpdir}/${asset}" "helm-docs ${VERSION} tarball"
verify_sha256 "${tmpdir}/${asset}" "${expected}"
tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}"
install -m 0755 "${tmpdir}/helm-docs" "${root}/${OUT}"
echo "installed ${OUT} (${VERSION} ${os}/${arch})"
