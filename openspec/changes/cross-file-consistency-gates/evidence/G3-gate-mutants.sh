#!/usr/bin/env bash
# Hand mutants of hack/test/consistency_rbac_test.sh (G3, task 3.2): each must make --self-test
# exit non-zero. Restores the gate from the first saved copy on exit.
# usage: bash G3-gate-mutants.sh   (from anywhere; edits the gate in place, then restores it)
set -uo pipefail
cd "$(dirname "$0")/../../../.." || exit 1
G=hack/test/consistency_rbac_test.sh
SAVE="$(mktemp)"
cp "$G" "$SAVE"
trap 'cp "$SAVE" "$G"; rm -f "$SAVE"' EXIT
run() {
  local label="$1" old="$2" new="$3"
  cp "$SAVE" "$G"
  python3 -c 'import sys; p,o,n=sys.argv[1:]; s=open(p).read(); assert o in s, o; open(p,"w").write(s.replace(o,n,1))' "$G" "$old" "$new" || { echo "MUTANT-NOT-LANDED $label"; return; }
  out="$(bash "$G" --self-test 2>&1)"; rc=$?
  echo "$label exit=$rc :: $(grep FAIL <<<"$out" | head -1)"
}
run "mode-agnostic exceptions" "if (mode, t) in exceptions:" "if any(k[1] == t for k in exceptions):"
run "resourceNames ignored" "        if rule.get(\"resourceNames\"):
            continue" "        pass"
run "reason not required" "if len(fields) < 5 or not fields[4].strip():" "if len(fields) < 4:"
run "stale not reported" "errors.append(f\"{exceptions_rel}:{n}: stale" "print(f\"{exceptions_rel}:{n}: stale"
run "empty render passes" "    if not roles:" "    if False:"
run "tenant mode not rendered" "\"tenant\": [\"--set\", \"tenantMode=true\"]" "\"tenant\": []"
run "crd scope unchecked" "if crd_scope.get(base) != \"Cluster\":" "if False:"
run "default counts the leader-election Role" 'if not doc or doc.get("kind") != COVERING_KIND[mode]:' 'if not doc or doc.get("kind") not in ("Role", "ClusterRole"):'
run "Role covers cluster-scoped" '    if kind == "Role" and is_cluster_scoped(g, r):
        return False' '    pass'
run "team-workload open list" 'if (t[0], resource) not in TEAM_WORKLOAD_KINDS or verb not in READ_VERBS:' 'if t[0] == "kollect.dev" or verb not in READ_VERBS:'
run "subresource not mapped to its kind" '    base = (g, r.split("/")[0])
    if g == "kollect.dev":' '    base = (g, r)
    if g == "kollect.dev":'
run "bare class token accepted" 'and not reason.split(":", 1)[1].strip():' 'and False:'
run "team-workload any verb" 'if (t[0], resource) not in TEAM_WORKLOAD_KINDS or verb not in READ_VERBS:' 'if (t[0], resource) not in TEAM_WORKLOAD_KINDS:'
cp "$SAVE" "$G"
bash "$G" >/dev/null && echo "restored gate passes"
