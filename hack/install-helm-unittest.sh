#!/usr/bin/env bash
# Install a pinned helm-unittest plugin release with SHA256-verified tarball download.
# Usage: hack/install-helm-unittest.sh <version>   (e.g. `task tools:helm-unittest`)
#
# WHY NOT `helm plugin install <git url> --version ...`: Helm 4 verifies plugin provenance by
# default and a git source cannot be verified, so that form now fails with "plugin source does not
# support verification". Upstream's answer is `--verify=false`, which leaves the git-clone install
# hook to download the binary and check it against a checksum file fetched without retry or
# protocol pinning. Instead this script fetches the per-platform release tarball through the
# shared hardened helper (hack/lib/fetch.sh), verifies its SHA256 against the release's checksum
# file -- the same trust model as every other hack/install-*.sh -- and unpacks it straight into
# Helm's plugin directory. The tarball is a complete plugin (plugin.yaml + binary), so no install
# hook runs and nothing is downloaded after verification. Helm's own GPG provenance check is not
# used: it needs the maintainer's public key in a legacy keyring, which this repo does not pin.
# Works unchanged under Helm 3 and Helm 4 (no `helm plugin install` flag differences involved).
set -euo pipefail

VERSION="${1:?version required (e.g. the Taskfile HELM_UNITTEST_VERSION)}"
VER="${VERSION#v}"

case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=macos ;;
  *)
    echo "unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

command -v helm >/dev/null 2>&1 || {
  echo "helm is not on PATH; install it first (hack/install-helm.sh)" >&2
  exit 1
}

asset="helm-unittest-${os}-${arch}-${VER}.tgz"
base_url="https://github.com/helm-unittest/helm-unittest/releases/download/v${VER}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${root}/hack/lib/verify-sha256.sh"
source "${root}/hack/lib/fetch.sh"

plugins_dir="$(helm env HELM_PLUGINS)"
if [[ -z "${plugins_dir}" ]]; then
  echo "helm env HELM_PLUGINS returned nothing; cannot locate the plugin directory" >&2
  exit 1
fi

installed="$(helm plugin list 2>/dev/null | awk '$1 == "unittest" {print $2}')"
if [[ "${installed}" == "${VER}" ]]; then
  echo "helm-unittest ${VER} already installed"
  exit 0
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# helm-unittest-checksum.sha lines read `<sha256> *./_dist/<asset>`.
fetch_to "${base_url}/helm-unittest-checksum.sha" "${tmpdir}/checksum.sha" "helm-unittest ${VERSION} checksums"
expected="$(awk -v file="./_dist/${asset}" '$2 == "*" file || $2 == file {print $1}' "${tmpdir}/checksum.sha")"
if [[ ! "${expected}" =~ ^[0-9a-f]{64}$ ]]; then
  echo "failed to resolve a checksum for ${asset} from ${base_url}/helm-unittest-checksum.sha" >&2
  exit 1
fi
fetch_to "${base_url}/${asset}" "${tmpdir}/${asset}" "helm-unittest ${VERSION} tarball"
verify_sha256 "${tmpdir}/${asset}" "${expected}"

mkdir -p "${tmpdir}/plugin"
tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}/plugin"
if [[ ! -f "${tmpdir}/plugin/plugin.yaml" ]]; then
  echo "${asset} has no plugin.yaml at its root; the release layout changed" >&2
  exit 1
fi

if [[ -n "${installed}" ]]; then
  echo "replacing helm-unittest ${installed} with ${VER}"
  helm plugin uninstall unittest
fi
mkdir -p "${plugins_dir}"
rm -rf "${plugins_dir}/unittest"
mv "${tmpdir}/plugin" "${plugins_dir}/unittest"

installed="$(helm plugin list 2>/dev/null | awk '$1 == "unittest" {print $2}')"
if [[ "${installed}" != "${VER}" ]]; then
  echo "helm plugin list reports unittest '${installed}' after install, expected ${VER}" >&2
  exit 1
fi
echo "installed helm-unittest ${VER} (${os}/${arch}) into ${plugins_dir}/unittest"
