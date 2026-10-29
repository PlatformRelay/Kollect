verdict: BLOCK
findings:
  - [P2] hack/test/renovate_policy_test.sh:183 — `yqv` suppresses shell-resolution errors into empty output, which selects default bash; reproduced with `defaults: [1,2]`. The `<unreadable>` fallback is unreachable.
matrix_disputes:
  - Row 2 — existing mutants do not exercise inherited defaults or shell-resolution errors; the claim that empty yq results always cause violations is false.
could_not_check:
  - Required clause-removal mutation verification, complete plain/self-test runs, and CWS execution: `/tmp` is read-only; both attempted guard modes failed at `mktemp`.
  - Runtime timeout and secret-isolation behavior; reviewed statically only.
summary: Shell selection for absent, explicit, and inherited bash checks out; T2+T7 still requires T2’s own message, and wiring appears correct. Mandatory execution verification remains blocked.
