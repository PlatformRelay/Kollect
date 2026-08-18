#!/usr/bin/env bash
# DIST-OLM-01: generate-olm-bundle must emit a registry+v1 bundle with all owned CRDs.
# DIST-OH-01: every owned CRD must carry an alm-examples entry.
# GATE-OWNED-01: spec.customresourcedefinitions.owned must cover config/crd/bases.
# DIST-OH-02: the generated bundle must pass the modern OperatorHub validator set locally.
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

# owned[] coverage gate (GATE-OWNED-01). The loop above proves each config/crd/bases file is
# COPIED into the bundle; nothing proved it is DECLARED in spec.customresourcedefinitions.owned.
# A new CRD base that never reaches owned[] therefore shipped silently: OLM would install the
# CRD but the console would not list the API, and alm-examples coverage would still pass because
# it compares against owned[] -- the very list that is missing the kind.
check_owned_covers_crd_bases() {
  local manifest="$1" label="$2"
  local base_kinds owned_kinds

  # Per-file yq: passing several files in one call interleaves "---" document separators
  # into the output, which would never match the owned[] list.
  base_kinds="$(while IFS= read -r crd_file; do
    yq '.spec.names.kind' "${crd_file}"
  done < <(find config/crd/bases -name 'kollect.dev_*.yaml' | sort) | LC_ALL=C sort -u)"
  owned_kinds="$(yq '.spec.customresourcedefinitions.owned[].kind' "${manifest}" | LC_ALL=C sort -u)"

  # Both extractions must be non-empty, otherwise a mistyped yq path or an empty
  # config/crd/bases would make the comparison below pass vacuously.
  [[ -n "${base_kinds}" ]] ||
    fail "${label}: extracted no kinds from config/crd/bases -- the owned[] coverage gate would pass vacuously"
  [[ -n "${owned_kinds}" ]] ||
    fail "${label}: extracted no kinds from spec.customresourcedefinitions.owned -- the owned[] coverage gate would pass vacuously"

  if [[ "${base_kinds}" != "${owned_kinds}" ]]; then
    printf 'dist olm bundle: %s spec.customresourcedefinitions.owned does not match config/crd/bases ("<" crd bases, ">" owned):\n' \
      "${label}" >&2
    diff <(printf '%s\n' "${base_kinds}") <(printf '%s\n' "${owned_kinds}") >&2 || true
    fail "${label}: declare every config/crd/bases kind under spec.customresourcedefinitions.owned in config/olm/template/manifests/kollect.clusterserviceversion.yaml (and give it an alm-examples entry)"
  fi

  pass "${label}: spec.customresourcedefinitions.owned covers all $(printf '%s\n' "${base_kinds}" | wc -l | tr -d ' ') CRD bases"
}

check_owned_covers_crd_bases "${TEMPLATE}" "CSV template"
check_owned_covers_crd_bases "${csv}" "generated CSV"

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

# --- DIST-OH-02: modern OperatorHub validator set ------------------------------------------
#
# Until now bundle defects were only discovered after pushing to the community-operators repo
# and reading someone else's pipeline output -- which is how the missing alm-examples were
# found. These checks pull that feedback local, and into CI.

# Shape gate for the Makefile recipe. `--select-optional` is a plain string flag, not a
# repeatable slice: passing it several times in ONE `bundle validate` invocation does not union
# the selectors, the LAST one wins and every earlier validator is silently dropped. A collapsed
# recipe would still exit 0 on a healthy bundle while only ever running one validator.
# Executable lines only -- the recipe's own comments name the anti-pattern they ban.
RECIPE="$(awk '$0 ~ /^validate-olm-bundle:/ {f=1; next} f && $0 !~ /^\t/ {exit} f' "${ROOT}/Makefile" |
  grep -v '^[[:space:]]*#' || true)"
[[ -n "${RECIPE}" ]] ||
  fail "could not extract the validate-olm-bundle recipe from Makefile -- the shape gate would pass vacuously"

# grep -F needs -e here: a pattern starting with -- is otherwise parsed as an option and the
# check silently never fires.
INVOCATIONS="$(printf '%s\n' "${RECIPE}" | grep -o -F -e 'bundle validate' | wc -l | tr -d ' ')"
SELECTORS="$(printf '%s\n' "${RECIPE}" | grep -o -F -e '--select-optional' | wc -l | tr -d ' ')"

[[ "${SELECTORS}" -ge 3 ]] ||
  fail "validate-olm-bundle must select at least 3 optional validators, found ${SELECTORS}"
[[ "${INVOCATIONS}" == "${SELECTORS}" ]] ||
  fail "validate-olm-bundle runs ${INVOCATIONS} 'bundle validate' invocation(s) for ${SELECTORS} --select-optional flag(s): give each validator its own invocation, or all but the last are silently dropped"

for validator in operatorhubv2 capabilities categories; do
  printf '%s\n' "${RECIPE}" | grep -Fq "name=${validator}" ||
    fail "validate-olm-bundle must select the ${validator} validator (operator-sdk's CLI name for the modern OperatorHub validator set)"
done

pass "validate-olm-bundle runs one validator per bundle validate invocation (${INVOCATIONS})"

# The validator run itself. operator-sdk is a hard requirement, exactly like yq and jq above:
# a gate that skips when the tool is missing reads as coverage that does not exist.
SDK="${ROOT}/bin/operator-sdk"
[[ -x "${SDK}" ]] || SDK="$(command -v operator-sdk || true)"
[[ -n "${SDK}" ]] || fail "operator-sdk is required for the OperatorHub validator gate. Install the pinned release with 'make operator-sdk' (equivalently: bash hack/install-operator-sdk.sh ./bin); upstream docs: https://sdk.operatorframework.io/docs/installation/"

# Validates the on-disk bundle directory, so no cluster and no registry are contacted.
make validate-olm-bundle VERSION="${VERSION}" ||
  fail "generated bundle failed operator-sdk bundle validate (operatorhubv2 / capabilities / categories)"

pass "generated bundle passes operatorhubv2 + capabilities + categories"

echo "All dist OLM bundle tests passed."
