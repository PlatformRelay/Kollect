## Unified verdict: CONCERNS (legs ok: 2/2)
| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | ERROR | Deleted monotonicity test was the sole guard for version survival across shard deletion/recreation; the invariant is now claimed in comments but untested, and loop.md's rejection rationale ("scenario unreachable without the dead method") is factually wrong — an in-package `delete(s.shards,…)` + `Upsert` reproduces it in five lines | internal/collect/store.go:42-48,179-186; internal/collect/store_test.go:103 (pre-deletion) | DeepSeek-V4.1-Flash, Qwen3.8-Flash-Next | 2/2 | 100 |
| 2 | WARNING | T3.md evidence file contradicts itself: header still `Status: FRAMED`/"Next action: implement" and matrix rows 3–14 `not-run`, while the Sensors table reports all pass and the commit is landed — an auditor cannot tell T3 was verified | openspec/changes/dead-exported-surface/evidence/T3.md:7-8,16-27 | DeepSeek-V4.1-Flash | 1/2 | 95 |

## Disagreements
- Entry 1 severity: DeepSeek rated NOTE (invariant currently unreachable; spec-set decision accepted, not re-litigated), Qwen rated WARNING (test reproducible in-package; reject rationale factually wrong) — merged on promotion rule.
- Qwen reports full-module `go build ./...` / `go vet ./...` / `go test ./internal/collect/...` all green on HEAD; DeepSeek listed build/vet as timed-out/unchecked — resolved in Qwen's favour, no defect.

## Nobody could check
- Task gates `task lint` / `task coverage` / `task spec:validate` / `task test` and any `//go:build`-tagged compile (task 8 / 3.4 gates) — neither leg ran them; entry 2's sensor-table gap is exactly here.
- CI-side coverage/arch-lint runs (`.go-arch-lint.yml`, `.github/workflows/ci.yaml`) — neither leg read them.
- loop.md R1 F7 full round-1 text — entry 1's "rationale is wrong" claim rests only on the round-2 wording and grep context.
- External consumers of `internal/collect` outside the module (package is internal; compile cannot prove the external half; repo-external anchor report unread).
