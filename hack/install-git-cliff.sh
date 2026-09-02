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

# NOTE: this installer verifies no digest -- upstream publishes checksums, but wiring them up is
# a separate change from routing the transport through the shared helper, and hack/test/
# ci_fetch_lib_hardening_test.sh records the gap explicitly so it cannot be forgotten.
fetch_to "${url}" "${tmpdir}/git-cliff.tgz" "git-cliff ${VERSION} tarball"
tar -xzf "${tmpdir}/git-cliff.tgz" -C "${tmpdir}"
install -m 0755 "${tmpdir}/git-cliff-${VER}/git-cliff" "${root}/${OUT}"
echo "installed ${OUT} (${VERSION} ${platform})"
