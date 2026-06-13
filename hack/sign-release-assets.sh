#!/usr/bin/env bash
# Sign GitHub Release artifacts with cosign keyless (Sigstore OIDC).
# Writes Sigstore bundles as *.sigstore.json — OSSF Scorecard Signed-Releases
# matches that extension (not the default .bundle from cosign sign-blob).
#
# Usage: hack/sign-release-assets.sh <dist-dir>
set -euo pipefail

DIST="${1:?dist directory required (e.g. dist)}"
cd "${DIST}"

shopt -s nullglob
files=(install-crds.yaml install.yaml kollect-*.tgz sbom*.json checksums.txt)

sign_blob() {
  local f="$1"
  local err
  err="$(mktemp)"

  echo "Signing ${f}"
  if cosign sign-blob --yes --bundle "${f}.sigstore.json" "${f}" 2>"${err}"; then
    rm -f "${err}"
    return 0
  fi

  # Release re-runs rebuild identical assets; Rekor rejects duplicate log entries (409).
  if grep -qE '409|already exists|createLogEntryConflict' "${err}"; then
    echo "Rekor entry already exists for ${f}; writing bundle without tlog upload"
    rm -f "${err}"
    cosign sign-blob --yes --tlog-upload=false --bundle "${f}.sigstore.json" "${f}"
    return $?
  fi

  cat "${err}" >&2
  rm -f "${err}"
  return 1
}

signed=0
for f in "${files[@]}"; do
  [[ -f "${f}" ]] || continue
  sign_blob "${f}"
  signed=$((signed + 1))
done

if [[ "${signed}" -eq 0 ]]; then
  echo "no release artifacts found to sign in ${DIST}" >&2
  exit 1
fi

echo "signed ${signed} release artifact(s) in ${DIST}/"
