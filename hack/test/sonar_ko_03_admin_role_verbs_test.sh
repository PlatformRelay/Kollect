#!/usr/bin/env bash
# Unit test for SEC-04c / KO-03 (SonarCloud): admin ClusterRole verbs must be
# explicit, not a wildcard. Asserts none of the six *_admin_role.yaml manifests
# grant a bare '*' verbs entry, and that the explicit admin verb list is present.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RBAC_DIR="${ROOT}/config/rbac"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

EXPECTED_VERBS=(get list watch create update patch delete deletecollection)

FILES=(
  "${RBAC_DIR}/kollectdatabasesink_admin_role.yaml"
  "${RBAC_DIR}/kollecteventsink_admin_role.yaml"
  "${RBAC_DIR}/kollectinventory_admin_role.yaml"
  "${RBAC_DIR}/kollectprofile_admin_role.yaml"
  "${RBAC_DIR}/kollectsnapshotsink_admin_role.yaml"
  "${RBAC_DIR}/kollecttarget_admin_role.yaml"
)

for f in "${FILES[@]}"; do
  [[ -f "${f}" ]] || fail "missing expected admin role manifest: ${f}"

  # No bare wildcard verbs entry ('*' as the sole/any verb list item).
  if grep -Eq "^\s*-\s*'\*'\s*$" "${f}"; then
    fail "${f} still grants a bare '*' verbs entry"
  fi

  # The explicit admin verb list must be present, in order, on a single verbs block.
  for verb in "${EXPECTED_VERBS[@]}"; do
    if ! grep -Eq "^\s*-\s*${verb}\s*$" "${f}"; then
      fail "${f} is missing explicit verb '${verb}'"
    fi
  done

  pass "$(basename "${f}") has explicit admin verbs, no wildcard"
done

echo "All sonar_ko_03 admin role verb tests passed."
