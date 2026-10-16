## Verdict: CLEAN

## Findings
- [NOTE] Round 1 added a 37th tenant exception (`selfsubjectaccessreviews create`) that the spec's own list doesn't name. Its reason text also differs from the one the spec prescribes. — `hack/test/testdata/consistency-rbac-exceptions.txt:25`
  Failure: spec.md:99 lists the tenant cluster-scoped exceptions as "namespaces, tokenreviews, subjectaccessreviews, `kollectcluster*`", each with the reason "tenant mode drops cluster scope". G3.md (3.1 decision text, around line 59) itself says that "adding exceptions the spec does not name is an allow-list widening". SSAR really is cluster-scoped, so the gate is right to need the line. But the operator never ruled on it, and the chart keeps a tenant Role rule that grants nothing (`charts/kollect/templates/role.yaml:10-12`).
  Fix: either add SSAR to the spec's list in the delta, or record it in decisions.md as covered by the "cluster-scoped triples" class. Optionally remove the dead SSAR rule from the tenant Role in a separate chart change.
  Confidence: 60
- [NOTE] Nothing tests that a Role cannot cover a cluster-scoped subresource. — `hack/test/consistency_rbac_test.sh:121-131`
  Failure: a gate mutant that drops `.split("/")[0]` in `is_cluster_scoped` survives the self-test. The only cluster-scoped mutant (`:457`) uses `namespaces`, which has no subresource. If someone later adds `kollectclusterinventories/status` to the tenant Role, the gate would count it as covered. The exception for it would then go stale, an author would delete it, and the gate would pass while the API returns Forbidden. The code is correct today; only the guard against a regression is missing.
  Fix: extend the `Role does not cover a cluster-scoped resource` mutant (or add one) to list `kollectclusterinventories/status` in the tenant Role and delete its `/status` exceptions. Add the matching hand mutant to `evidence/G3-gate-mutants.sh`.
  Confidence: 75
- [NOTE] A tenant reason that is only the class token (`cluster-scoped:` with nothing after the colon) is accepted as a reason. — `hack/test/consistency_rbac_test.sh:176,186`
  Failure: `tenant core namespaces get cluster-scoped:` passes the "every line needs a reason" check. The class carries some meaning, so the impact is small.
  Fix: require non-empty text after the class prefix, and add a mutant for it.
  Confidence: 70
- [NOTE] The evidence is slightly inconsistent. G3.md 3.2 says "mutants 5 and 16", which doesn't match any numbering in the script. The matrix rows run 22, 23, 24, then 21. "Real drift" says 30 cluster-scoped exceptions, then adds a 37th, while row 13 says 31. — `openspec/changes/cross-file-consistency-gates/evidence/G3.md` (3.2 design decisions, matrix)
  Fix: renumber the rows and cite mutants by label.
  Confidence: 85

Checked against the code and found correct:
- **Round-1 fixes:**
  - `COVERING_KIND` limits default coverage to ClusterRoles and tenant coverage to Roles (`:80,225`).
  - `covers` refuses cluster-scoped resources for a Role (`:130`).
  - `TEAM_WORKLOAD_KINDS` closes the team-workload class and matches the spec's "today" list (`:85,195`).
  - The ABSENT probes work (`:297`).
  - The gate mutants are committed.
- **Exceptions file:** recounted by hand from `config/rbac/role.yaml` and both templates. role.yaml has 113 triples. The tenant render misses 31 cluster-scoped and 6 workload-read triples, 37 in all, and the file has exactly those 37 lines. Each class matches the CRD scope or the built-in list.
- **Leader-election Role:** in tenant mode it covers configmaps, and the error message's role list matches the sentinel at `:458`.
- **Mode isolation:** each line excuses only its own mode, in both directions. Stale detection is filtered per mode (`:242`). Duplicate and unknown-mode lines fail, and so does a missing file.
- **Self-test:** 24 `mutant_rejected` calls, a no-op control and 2 green controls, 27 `ok` lines in all, matching the sensors table. Each asserts the failure message as well as the exit code, and every sentinel string matches the message the gate prints.

## Could not check
- I did not run the gate, `--self-test` or `evidence/G3-gate-mutants.sh`; the results above come from G3.md 3.2.
- That `config/crd/bases` marks `kollectclusterscopes`, `kollectclusterinventories` and `kollectclustertargets` as `scope: Cluster`, and `kollectprofiles` as namespaced: I didn't read the CRDs.
- The CFC-6 "required job on pull_request" wiring is deferred to task 6.1 and isn't in this range.
- Whether the tenant Role should also cover the `watchNamespaces` other than the release namespace (`values.yaml:38`): the spec doesn't ask for it, and I didn't assess it.
- `docs/examples/team-operator.md` §3 and the KollectTarget controller markers: not read.
