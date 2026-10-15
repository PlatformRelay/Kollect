## Verdict: CONCERNS

## Findings
- [WARNING] The gate puts the rules of every rendered manager Role and ClusterRole into one list and never checks scope. In default mode, the namespaced leader-election Role can therefore stand in for cluster-wide grants. — `hack/test/consistency_rbac_test.sh:207`, `:213`
  Failure: someone deletes `configmaps` from the ClusterRole's `[configmaps, namespaces, secrets]` rule (`charts/kollect/templates/clusterrole.yaml`), or deletes the `events` rule. `role.yaml` grants `configmaps get/list/watch` and `events create/patch` cluster-wide. The gate still sees them as covered by `role-leader-election.yaml`, which is a Role in `{{ .Release.Namespace }}` that grants configmaps (all verbs) and events. So the gate passes, but a Helm install is Forbidden on configmaps and events in every other namespace. That is exactly the failure CFC-3 exists to catch. No mutant covers this; mutant 6 only touches tokenreviews.
  Fix: in default mode, check the ClusterRole's rules separately from the namespaced Roles (a namespaced Role only counts toward namespaced triples, and only for the release namespace). Add a mutant that drops `configmaps` from the ClusterRole and expects the "default: role.yaml grants core configmaps get" message.
  Confidence: 85
- [WARNING] The `team-workload:` class accepts a get/list/watch of any non-kollect.dev kind, including reconciler needs such as `core secrets`. The amended scenario and the evidence say "any other namespaced triple missing from the Role fails". Adding one exception line undoes that. — `hack/test/consistency_rbac_test.sh:173-176`
  Failure: the tenant Role drops `secrets` (the case mutant 11 tests) and the author adds `tenant core secrets get|list|watch team-workload: x`. Those lines pass the class check, are not stale, and the gate is green. Mutant 9 only tries `cluster-scoped:` for secrets, so the evidence's claim "not undone by adding a line" (G3.md 3.2, design decisions) is only half proven.
  Fix: limit `team-workload:` to a fixed set of workload GVKs, for example the ones the KollectTarget controller markers name (`internal/controller/kollecttarget_controller.go:41-42`), or at least reject core-group reconciler kinds (secrets, configmaps, events). Add a `tenant core secrets get team-workload:` mutant.
  Confidence: 80
- [NOTE] The gate counts a cluster-scoped resource as covered when it is listed in a namespaced Role. The tenant Role lists `selfsubjectaccessreviews create`, so that `role.yaml` triple needs no `cluster-scoped:` exception even though a Role cannot grant it. — `charts/kollect/templates/role.yaml` (first rule); gate `:109-118`
  Failure: no harm today, because `system:basic-user` already grants SSAR. But the same blind spot would hide `namespaces` or `tokenreviews` if they were added to the tenant Role: the exception would go stale, the author would delete it, and the gate would pass while the API returns Forbidden.
  Fix: a namespaced Role should never cover a resource in `CLUSTER_SCOPED_BUILTINS` or a Cluster-scoped CRD.
  Confidence: 60
- [NOTE] Several self-test probes prove nothing. Because each mutant also asserts its expected FAIL message, a no-op mutation would still red the self-test, so this is cosmetic. `'kollect'` in clusterrole.yaml (`:387`) and `'^tenantMode:'` in values.yaml (`:397`) hold whether or not the mutation landed. — `hack/test/consistency_rbac_test.sh:387,397`
  Fix: probe for the effect instead, for example `test ! -e` on the removed file.
  Confidence: 70
- [NOTE] The evidence file contradicts itself. The top-level matrix row 3 still says "not-run (BLOCKED)", while section 3.2 reports pass. Row 19 relies on `/tmp/claude-g3-gate-mutants.sh`, which is not committed, so nobody can re-run that claim. — `openspec/changes/cross-file-consistency-gates/evidence/G3.md` (top-level verification matrix, row 3; 3.2 matrix, row 19)
  Fix: update row 3 to point at section 3.2, and commit or describe the gate mutants in enough detail to reproduce them.
  Confidence: 90

**What I checked and found OK against the code:**
- Mode-per-line parsing; reason required, including the trailing-blank case.
- Unknown mode, duplicate lines and a missing exceptions file all fail.
- Stale detection in both directions.
- Cluster-scoped check against CRD scope.
- `resourceNames` rules cover nothing; `*` matches.
- Empty-scan guards on both sides.
- Filtering by `# Source:` instead of `--show-only`.
- The 36 exception lines match the gaps between the tenant Role and `role.yaml` (30 cluster-scoped, 6 workload).
- Self-tests check the FAIL message as well as the exit code; temp dirs are cleaned by the EXIT trap.

## Could not check
- I did not run the gate or the self-test (read-only leg); the results above come from G3.md 3.2.
- `/tmp/claude-g3-gate-mutants.sh` and `evidence/G3-rbac-probe.py`: outside scope or not in this diff.
- That `config/crd/bases` marks the `kollectcluster*` kinds as `scope: Cluster`: I did not read the CRDs.
- `docs/examples/team-operator.md` §3 and the `kollecttarget_controller.go` markers: not read.
- CI wiring (CFC-6 "required job"): deferred to task 6.1, out of this range.
