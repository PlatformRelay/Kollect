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
url="https://github.com/norwoodj/helm-docs/releases/download/${VERSION}/${asset}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/fetch.sh
source "${root}/hack/lib/fetch.sh"

mkdir -p "$(dirname "${root}/${OUT}")"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# NOTE: this installer verifies no digest -- see the matching note in hack/install-git-cliff.sh.
fetch_to "${url}" "${tmpdir}/helm-docs.tgz" "helm-docs ${VERSION} tarball"
tar -xzf "${tmpdir}/helm-docs.tgz" -C "${tmpdir}"
install -m 0755 "${tmpdir}/helm-docs" "${root}/${OUT}"
echo "installed ${OUT} (${VERSION} ${os}/${arch})"
