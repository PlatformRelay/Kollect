#!/usr/bin/env bash
# DIST-OLM-01: generate-olm-bundle must emit a registry+v1 bundle with all owned CRDs.
# DIST-OH-01: every owned CRD must carry an alm-examples entry.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
  printf 'dist olm bundle: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

TEMPLATE="${ROOT}/config/olm/template/manifests/kollect.clusterserviceversion.yaml"
[[ -f "${TEMPLATE}" ]] || fail "${TEMPLATE} is missing"

# yq drives the CSV/role.yaml RBAC drift gate, jq the alm-examples coverage gate.
# Both are hard requirements: without them there is nothing to compare and the
# gates below would be skipped rather than enforced.
command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required for the CSV/role.yaml RBAC drift gate"
command -v jq >/dev/null 2>&1 ||
  fail "jq is required for the CSV alm-examples coverage gate"

grep -Fq 'make generate-olm-bundle' "${ROOT}/Makefile" ||
  fail "Makefile must define generate-olm-bundle target"

VERSION="9.9.9-test"
IMAGE_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
BUNDLE_DIR="dist/olm-bundle/${VERSION}"

rm -rf "${BUNDLE_DIR}"
make generate-olm-bundle VERSION="${VERSION}" IMAGE_DIGEST="${IMAGE_DIGEST}"

for path in \
  "${BUNDLE_DIR}/manifests/kollect.clusterserviceversion.yaml" \
  "${BUNDLE_DIR}/metadata/annotations.yaml"; do
  [[ -f "${path}" ]] || fail "missing ${path}"
done

while IFS= read -r crd_file; do
  base="$(basename "${crd_file}")"
  [[ -f "${BUNDLE_DIR}/manifests/${base}" ]] || fail "bundle missing CRD ${base}"
done < <(find config/crd/bases -name 'kollect.dev_*.yaml' | sort)

csv="${BUNDLE_DIR}/manifests/kollect.clusterserviceversion.yaml"
grep -Fq "kollect.v${VERSION}" "${csv}" || fail "CSV metadata.name must include version"
grep -Fq "ghcr.io/platformrelay/kollect@${IMAGE_DIGEST}" "${csv}" ||
  fail "CSV must digest-pin the controller image"

# alm-examples coverage gate (DIST-OH-01). The OpenShift console and operatorhub.io
# pre-fill "Create instance" from metadata.annotations["alm-examples"]; an owned CRD
# with no entry hands the user a blank YAML editor, and the OperatorHub hosted
# validator warns "provided API should have an example annotation". Comparing the two
# sets is what makes a newly owned CRD unable to ship without an example -- the two
# `grep -Fq 'kind: Kollect...'` checks this replaces matched the *owned* list (plain
# YAML), never the JSON annotation, so they never tested alm-examples at all.
#
# Both the template (the file an author edits) and the generated bundle CSV (what
# upstream actually validates) are checked: generation is a sed pass, so a
# template-only assertion would not prove the annotation survives it.
check_alm_examples() {
  local manifest="$1" label="$2"
  local examples example_kinds owned_kinds example_count unique_count

  examples="$(yq '.metadata.annotations."alm-examples"' "${manifest}")"
  [[ -n "${examples}" && "${examples}" != "null" ]] ||
    fail "${label}: metadata.annotations[\"alm-examples\"] is missing or empty"

  printf '%s' "${examples}" | jq empty ||
    fail "${label}: alm-examples is not valid JSON"
  printf '%s' "${examples}" | jq -e 'type == "array"' >/dev/null ||
    fail "${label}: alm-examples must be a JSON array"

  example_kinds="$(printf '%s' "${examples}" | jq -r '.[].kind' | LC_ALL=C sort)"
  owned_kinds="$(yq '.spec.customresourcedefinitions.owned[].kind' "${manifest}" | LC_ALL=C sort)"

  # Both extractions must be non-empty, otherwise a mistyped yq/jq path would make
  # the comparison below pass vacuously and the gate would never fire.
  [[ -n "${example_kinds}" ]] ||
    fail "${label}: extracted no kinds from alm-examples -- the coverage gate would pass vacuously"
  [[ -n "${owned_kinds}" ]] ||
    fail "${label}: extracted no kinds from spec.customresourcedefinitions.owned -- the coverage gate would pass vacuously"

  # One example per owned CRD: a duplicated kind must not mask a missing one.
  example_count="$(printf '%s\n' "${example_kinds}" | wc -l | tr -d ' ')"
  unique_count="$(printf '%s\n' "${example_kinds}" | LC_ALL=C sort -u | wc -l | tr -d ' ')"
  [[ "${example_count}" == "${unique_count}" ]] ||
    fail "${label}: alm-examples has duplicate kinds (${example_count} entries, ${unique_count} distinct)"

  if [[ "${example_kinds}" != "${owned_kinds}" ]]; then
    printf 'dist olm bundle: %s alm-examples do not cover spec.customresourcedefinitions.owned ("<" alm-examples, ">" owned):\n' \
      "${label}" >&2
    diff <(printf '%s\n' "${example_kinds}") <(printf '%s\n' "${owned_kinds}") >&2 || true
    fail "${label}: add one alm-examples entry per owned CRD in config/olm/template/manifests/kollect.clusterserviceversion.yaml"
  fi

  pass "${label}: alm-examples covers all ${example_count} owned CRDs"
}

check_alm_examples "${TEMPLATE}" "CSV template"
check_alm_examples "${csv}" "generated CSV"

# RBAC drift gate: the CSV clusterPermissions are a hand copy of the
# controller-gen-generated config/rbac/role.yaml. Without this gate the next
# +kubebuilder:rbac marker change would silently ship an under-privileged bundle
# that 403s at runtime on OperatorHub while the Helm chart keeps working.
ROLE="${ROOT}/config/rbac/role.yaml"
[[ -f "${ROLE}" ]] || fail "${ROLE} is missing"

# Order-insensitive, key-order-insensitive normalisation of a PolicyRule list.
RULE_NORMALIZE='map({"apiGroups": (.apiGroups // [] | sort), "nonResourceURLs": (.nonResourceURLs // [] | sort), "resourceNames": (.resourceNames // [] | sort), "resources": (.resources // [] | sort), "verbs": (.verbs // [] | sort)}) | .[]'
CSV_SA="kollect-controller-manager"

ROLE_RULES="$(yq eval -o=json -I=0 ".rules | ${RULE_NORMALIZE}" "${ROLE}" | LC_ALL=C sort)"
CSV_RULES="$(yq eval -o=json -I=0 \
  ".spec.install.spec.clusterPermissions[] | select(.serviceAccountName == \"${CSV_SA}\") | .rules | ${RULE_NORMALIZE}" \
  "${csv}" | LC_ALL=C sort)"

# Both extractions must be non-empty, otherwise a mistyped yq path would make the
# comparison below pass vacuously and the gate would never fire.
[[ -n "${ROLE_RULES}" ]] ||
  fail "extracted no rules from ${ROLE} -- the drift gate would pass vacuously"
[[ -n "${CSV_RULES}" ]] ||
  fail "extracted no clusterPermissions rules for serviceAccountName ${CSV_SA} from ${csv} -- the drift gate would pass vacuously"

if [[ "${ROLE_RULES}" != "${CSV_RULES}" ]]; then
  printf 'dist olm bundle: CSV clusterPermissions drifted from config/rbac/role.yaml ("<" role.yaml, ">" CSV):\n' >&2
  diff <(printf '%s\n' "${ROLE_RULES}") <(printf '%s\n' "${CSV_RULES}") >&2 || true
  fail "sync config/olm/template/manifests/kollect.clusterserviceversion.yaml clusterPermissions with config/rbac/role.yaml (run 'make manifests' first if role.yaml itself is stale)"
fi

pass "CSV clusterPermissions match config/rbac/role.yaml"

meta="${BUNDLE_DIR}/metadata/annotations.yaml"
grep -Fq 'operators.operatorframework.io.bundle.package.v1: kollect' "${meta}" ||
  fail "bundle package must be kollect"
grep -Fq 'operators.operatorframework.io.bundle.channels.v1: stable' "${meta}" ||
  fail "bundle channel must be stable"

pass "generate-olm-bundle produced complete bundle for ${VERSION}"

echo "All dist OLM bundle tests passed."
