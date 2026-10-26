verdict: REQUEST_CHANGES
findings:
  - [P1] hack/test/renovate_policy_test.sh:169 — Missing shell defaults to explicit Bash semantics in the guard, but GitHub's implicit Bash lacks pipefail. An empty-token branch containing `echo RENOVATE_TOKEN; false | true` passes T2 yet succeeds in CI; require explicit Bash or reproduce the effective shell.
  - [P2] hack/test/renovate_policy_test.sh:145 — `timeout 20` has no kill-after deadline; a body ignoring TERM can hang the guard indefinitely.
  - [P2] hack/test/renovate_policy_test.sh:343 — T7 permits RENOVATE_TOKEN on every run-only step, whereas its case text permits only the T2 step; the documented relaxation weakens the required clause.
matrix_disputes:
  - Row 2 — Shell mutants miss implicit Bash; direct execution reproduced guard empty/set statuses 1/0 versus runner statuses 0/0.
  - Row 7 — Existing mutants do not test RENOVATE_TOKEN on an unrelated run-only step.
could_not_check:
  - Clause-removal mutations, full plain/self-test runs and CWS execution: enforced read-only filesystem also blocks /tmp; plain execution stopped at T2's mktemp.
summary: Wiring and fixture structure match the task, but T2 can falsely certify fail-closed behavior; the intentional T1–T8 red is not a defect.
