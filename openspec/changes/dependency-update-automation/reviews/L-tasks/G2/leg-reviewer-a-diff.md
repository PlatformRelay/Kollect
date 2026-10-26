verdict: REQUEST_CHANGES
findings:
  - [P2] hack/test/renovate_policy_test.sh:794 — T16 searches for the expected image anywhere in argv; `run --rm --entrypoint renovate-config-validator alpine:latest "$T16_IMAGE" --strict --no-global renovate.json` incorrectly returns `repo --no-global --strict`. Parse Docker’s actual image position and add this decoy mutant.
matrix_disputes:
  - Row 7, “from that exact image” — reproduced acceptance of alpine:latest as the actual image; the existing hardcoded-image mutant tests absence of the expected string, not its position.
could_not_check:
  - Full self-test, clause-removal mutations, and real fixture validation: the enforced sandbox rejects temporary writes, including mktemp under /tmp.
summary: Confirmed expected T10–T15/T17 outcomes and clean shell syntax; T16 has a demonstrated false green, and mutation verification remains incomplete.
