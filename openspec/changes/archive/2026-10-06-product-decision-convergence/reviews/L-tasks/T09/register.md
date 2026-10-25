## Unified verdict: CONCERNS  (legs ok: 2/2)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | HIGH | Truth-up incomplete: residual sites still present the removed `cli` engine as live/selectable, so ADR-0803's "every site truthed up" claim overstates | prune_tree.go:96, prune_target.go:147, cleanup.go:294 (comments); ADR-0104:117, ADR-0407:57, ADR-0101:26 | 2 | 2 | 100 |

## Disagreements
- None material on the gates (both verified axis removal, KEX implementation in x/crypto v0.57.0, unreachable mirror surface, CRD/golden consistency); one interpretive split: DeepSeek reads the residual sites as a docs-claim defect, Qwen reads the ADR exclusions as "arguably intentional" — both still flag the overclaim.

## Nobody could check
- Integration suite never executed (no Docker on either leg): the 9 deleted + 6 surviving locks' vacuity argument is read, not run.
- Recorded gates trusted from evidence/T09.md as written (test-integration, docs_launch_truth, spec:validate, helm-docs verify, task verify); DeepSeek additionally ran only `go vet -tags integration`.
- Coverage gate/baseline (`task coverage`) not run; the test deletions could have moved it.
- Pipeline-CLI runtime path not exercised against a git sink (Dockerfile.pipeline and callers read only).
