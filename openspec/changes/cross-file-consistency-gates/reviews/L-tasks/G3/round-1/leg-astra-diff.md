## Verdict
verdict: BLOCK
findings:

  - [P0] hack/test/consistency_rbac_test.sh:174 — Although exceptions require review, `team-workload:` accepts core secrets/PVC reads; adding such exceptions can hide missing reconciler grants, violating CFC-3’s tenant restriction.
  - [P1] hack/test/consistency_rbac_test.sh:204 — Although manager roles include both kinds generally, tenant coverage explicitly requires Roles; changing the tenant Role to ClusterRole still passes while its RoleBinding references the now-absent Role.
matrix_disputes:

  - Row 12 — Rejecting writes and kollect.dev kinds does not prove workload-only exceptions; non-workload core reads pass its classification.
  - Row 18 — “Self-test per invariant” overstates coverage: neither bypass above has a negative control.
could_not_check:

  - Self-test and scratch mutations: mktemp failed because /tmp is read-only; historical seven gate mutations and cleanup counts were not independently reproduced.
summary: Plain guard, both existing RBAC guards, and shellcheck pass using system Python; current exceptions match the ruling, but tenant enforcement has gaps.
