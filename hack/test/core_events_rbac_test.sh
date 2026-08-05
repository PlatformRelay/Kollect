#!/usr/bin/env bash
# Regression lock: controller-runtime EventRecorder writes core/v1 Events
# (apiGroup ""), not events.k8s.io. Manager RBAC must grant core events
# create/patch; events.k8s.io-only grants cause Forbidden and break
# e2e-finalizer-cleanup SinkNotFound Warning asserts (nightly 30979399720).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROLE="${ROOT}/config/rbac/role.yaml"
CLUSTERROLE_TMPL="${ROOT}/charts/kollect/templates/clusterrole.yaml"
ROLE_TMPL="${ROOT}/charts/kollect/templates/role.yaml"
CONTROLLERS=(
  "${ROOT}/internal/controller/kollecttarget_controller.go"
  "${ROOT}/internal/controller/kollectclustertarget_controller.go"
  "${ROOT}/internal/controller/kollectinventory_controller.go"
  "${ROOT}/internal/controller/kollectclusterinventory_controller.go"
)

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

[[ -f "${ROLE}" ]] || fail "missing ${ROLE}"
[[ -f "${CLUSTERROLE_TMPL}" ]] || fail "missing ${CLUSTERROLE_TMPL}"
[[ -f "${ROLE_TMPL}" ]] || fail "missing ${ROLE_TMPL}"

# Generated manager ClusterRole must grant core events create+patch.
# Match a rule block with apiGroups: [""] (or - "") and resources including events
# with create and patch. Reject events.k8s.io as the sole events grant for the manager.
if grep -Eq '^\s*-\s*events\.k8s\.io\s*$' "${ROLE}"; then
  fail "${ROLE}: still grants events.k8s.io (recorder needs core apiGroup \"\")"
fi

# Controllers use kubebuilder markers; empty groups="" must appear for events.
for f in "${CONTROLLERS[@]}"; do
  [[ -f "${f}" ]] || fail "missing controller ${f}"
  if grep -Eq 'kubebuilder:rbac:groups=events\.k8s\.io,resources=events' "${f}"; then
    fail "$(basename "${f}"): events RBAC marker still uses events.k8s.io"
  fi
  if ! grep -Eq 'kubebuilder:rbac:groups="",resources=events,verbs=create;patch' "${f}"; then
    fail "$(basename "${f}"): missing core events kubebuilder marker groups=\"\""
  fi
  pass "$(basename "${f}") core events marker"
done

# YAML: ensure a core "" events create/patch rule exists.
# controller-gen emits:
# - apiGroups:
#   - ""
#   resources:
#   - events
#   verbs:
#   - create
#   - patch
python3 - "${ROLE}" <<'PY' || fail "config/rbac/role.yaml missing core events create/patch"
import sys, yaml
path = sys.argv[1]
doc = yaml.safe_load(open(path))
rules = doc.get("rules") or []
ok = False
for r in rules:
    groups = r.get("apiGroups") or []
    resources = set(r.get("resources") or [])
    verbs = set(r.get("verbs") or [])
    if "" in groups and "events" in resources and {"create", "patch"} <= verbs:
        ok = True
        break
if not ok:
    sys.exit(1)
print("ok - config/rbac/role.yaml core events create/patch")
PY

# Helm templates: replace events.k8s.io with core "" (not duplicate).
for tmpl in "${CLUSTERROLE_TMPL}" "${ROLE_TMPL}"; do
  if grep -Fq 'apiGroups: [events.k8s.io]' "${tmpl}"; then
    fail "$(basename "${tmpl}"): still grants events.k8s.io"
  fi
  if ! grep -Fq 'apiGroups: [""]' "${tmpl}"; then
    fail "$(basename "${tmpl}"): no core apiGroups [\"\"]"
  fi
  # events rule must sit under a core "" block with create, patch
  if ! awk '
    /apiGroups: \[""\]/ { core=1; next }
    /apiGroups:/ { core=0 }
    core && /resources: \[events\]/ { ev=1 }
    core && ev && /verbs: \[create, patch\]/ { found=1 }
    END { exit found ? 0 : 1 }
  ' "${tmpl}"; then
    fail "$(basename "${tmpl}"): missing core \"\" events create/patch rule"
  fi
  pass "$(basename "${tmpl}") core events create/patch"
done

echo "All core_events_rbac tests passed."
