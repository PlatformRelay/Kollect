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
mkdir -p "$(dirname "${root}/${OUT}")"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

curl -fsSL "${url}" -o "${tmpdir}/git-cliff.tgz"
tar -xzf "${tmpdir}/git-cliff.tgz" -C "${tmpdir}"
install -m 0755 "${tmpdir}/git-cliff-${VER}/git-cliff" "${root}/${OUT}"
echo "installed ${OUT} (${VERSION} ${platform})"
