#!/usr/bin/env bash
# Regression lock: scope.LoadCluster lists KollectClusterScope on every
# KollectClusterTarget/KollectClusterInventory reconcile, and both cluster-kind
# webhooks call it with failurePolicy=fail. Without get/list/watch on
# kollectclusterscopes the manager cache cannot sync that type, so reconcile
# errors and admission rejects every cluster-kind write. The grant was missing
# from the generated role and the chart until this lock landed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROLE="${ROOT}/config/rbac/role.yaml"
CLUSTERROLE_TMPL="${ROOT}/charts/kollect/templates/clusterrole.yaml"
CONTROLLERS=(
  "${ROOT}/internal/controller/kollectclustertarget_controller.go"
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

# Controllers that reach scope.LoadCluster must carry the marker; controller-gen
# aggregates them into the single manager role.
for f in "${CONTROLLERS[@]}"; do
  [[ -f "${f}" ]] || fail "missing controller ${f}"
  if ! grep -Eq 'kubebuilder:rbac:groups=kollect\.dev,resources=kollectclusterscopes,verbs=get;list;watch' "${f}"; then
    fail "$(basename "${f}"): missing kollectclusterscopes get;list;watch marker"
  fi
  pass "$(basename "${f}") kollectclusterscopes marker"
done

python3 - "${ROLE}" <<'PY' || fail "config/rbac/role.yaml missing kollectclusterscopes get/list/watch"
import sys, yaml
doc = yaml.safe_load(open(sys.argv[1]))
for rule in doc.get("rules") or []:
    groups = set(rule.get("apiGroups") or [])
    resources = set(rule.get("resources") or [])
    verbs = set(rule.get("verbs") or [])
    if "kollect.dev" in groups and "kollectclusterscopes" in resources and {"get", "list", "watch"} <= verbs:
        print("ok - config/rbac/role.yaml kollectclusterscopes get/list/watch")
        break
else:
    sys.exit(1)
PY

# The chart ClusterRole is hand-maintained and only rendered outside tenantMode,
# which is exactly where the cluster kinds are served.
if ! grep -Eq '^\s*resources: \[[^]]*kollectclusterscopes[^]]*\]' "${CLUSTERROLE_TMPL}"; then
  fail "$(basename "${CLUSTERROLE_TMPL}"): no kollectclusterscopes in any resources list"
fi
if ! awk '
  /resources: \[[^]]*kollectclusterscopes[^]]*\]/ { found=1; next }
  found && /verbs: \[get, list, watch\]/ { ok=1 }
  found && /verbs:/ && !ok { exit 1 }
  END { exit ok ? 0 : 1 }
' "${CLUSTERROLE_TMPL}"; then
  fail "$(basename "${CLUSTERROLE_TMPL}"): kollectclusterscopes rule lacks verbs [get, list, watch]"
fi
pass "$(basename "${CLUSTERROLE_TMPL}") kollectclusterscopes get/list/watch"

echo "All cluster_scope_rbac tests passed."
