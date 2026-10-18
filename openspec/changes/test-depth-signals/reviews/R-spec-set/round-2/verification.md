# R spec-set review, round 2: verification

Change `test-depth-signals`, spec set at `9a42d7591` (round-1 fixes applied), reviewed 2026-10-18.
Questions: are round-1 findings V1-V16 closed; is anything new broken by the fixes; would
executing `tasks.md` deliver the spec? This is the last spec-set round (allowed because round 1
accepted a CRITICAL). Verified mechanical fixes are applied with no further review.

## Legs

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (spec lens, cold brief `brief-astra.md`) | `leg-astra-spec.md` | 0 | BLOCK |
| claude/opus:spec (fanout.sh, `--no-unify`) | `leg-claude-opus-spec.md` | 0 (165 s) | CLEAN |

Both legs ran, in parallel, waited on by PID. No stand-in.

## Round-1 closure

Astra: V1-V3, V7, V9-V15 closed; V4, V5, V6, V8, V16 partly (its R2-1, R2-2, R2-3, R2-1, R2-6).
Opus: all closed. After the fixes below, all of V1-V16 are closed (V15 by its recorded rejection,
V17 still deferred to task 1.1).

## Findings

| Id | Legs | Sev (leg / verified) | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| R2-1 | astra | CRITICAL / WARNING | yes | S9 (empty assets) cannot kill "drop `make setup-envtest`" or "drop the export" | fix |
| R2-2 | astra | CRITICAL / CRITICAL | yes | allow-list cases never test a missing, repeated or reordered allowed line | fix |
| R2-3 | astra | WARNING / WARNING | yes | "the message names the marker" can match replayed `go test` output | fix |
| R2-4 | astra, opus (NOTE 5) | WARNING / WARNING | yes | no seed or fixture seam for S1, S3, S4 | fix |
| R2-5 | astra | WARNING / WARNING | yes | proposal says Ginkgo runs in "default order"; it randomizes top-level containers | fix |
| R2-6 | astra, opus (NOTE 1) | NOTE / NOTE | yes | reporter citations stop before their assertions; cited case covers close only | fix |
| R2-7 | opus | NOTE / NOTE | yes | "Seed reproducible" gives `-count=3`, which Ginkgo refuses | fix |
| R2-8 | opus | NOTE / NOTE | yes | `<pkg>/testdata/fuzz/` does not fit matrix values like `./internal/aggregate/...` | fix |
| R2-9 | opus | NOTE / NOTE | yes | `kubectl get` rule does not say what happens to non-short names | fix |

### R2-1: envtest provisioning not isolated

Verified by reading `tasks.md` S9 and 2.5 at `9a42d7591`: a fake `make` printing nothing fails
the script whether or not `make setup-envtest` ran or the value was exported, so those two mutants
survive. `Makefile:65-66` (`echo-kubebuilder-assets: envtest`, then `setup-envtest use ... -p
path`) confirms that resolving the path does not prove the explicit setup call. Downgraded to
WARNING: 2.5 requires every mutant red, so the self-test would expose the survivor during the task
rather than ship green. Fix: S9a (setup called before resolution; every `go test` sees the
exported path) and S9b (empty path fails); 2.5 has separate mutants for setup, export and the
empty check.

### R2-2: the sequence is not tested

Verified: F7, F12 and F13 cover unknown text, an extra unknown line and another target; a runner
checking unordered membership of allowed lines passes all three while violating "in order".
CRITICAL by the brief's definition (a guard green while the spec is violated). Fix: TDS-1 now says
each line exactly once, in order, and a missing, repeated or reordered line is not the flake; F14
(truncated before `exit status 1`) and F15 (a repeated line; `FAIL` and `context deadline
exceeded` swapped); 1.5 mutants "stop requiring every line" (F14) and "membership instead of
sequence" (F15).

### R2-3: diagnostics versus replayed output

Verified: with the allow-list, a deadline fixture plus a deny marker is rejected even when the
marker check is gone, and the marker text stays in the log the runner echoes, so an assertion on
the whole output would not kill the drop-marker mutant. Fix: TDS-1 defines the runner's own
diagnostics as lines starting `::error::fuzz-retry:` (`deny marker <marker>`,
`unrecognised failure text`, `no fuzz target ran`); 1.1 says cases match that line alone.

### R2-4: seed and fixture seam

Verified: S1 ran "on the fixture with that seed", while the stub runs on `go list ./...`, the
fixture is outside discovery, and TDS-2 named no seed input. Fix: TDS-2 adds `SHUFFLE_SEED`
(decimal, else fail; chosen when unset) and states that discovery is only `go list ./...`, so tests
reach the fixture with a fake `go` on `PATH`; the workflow sets no `SHUFFLE_SEED` (S11 and a 2.5
mutant, opus's point that a leftover override would fix the nightly's seed). S2 calls `go test`
directly. The stub ignores `SHUFFLE_SEED`, so S1 stays red against it.

### R2-5: Ginkgo ordering

Verified in Ginkgo v2.33.0: `types/config.go:50` seeds `RandomSeed` from `time.Now().Unix()`;
`:287-288` documents that by default it randomizes the top-level containers. Fix: proposal
*Non-goals* and the TDS-2 paragraph now say Go's `-shuffle` does not control ordering inside
`RunSpecs`, Ginkgo keeps its own seed, and reproducing a Ginkgo failure may need that seed. The
non-goal (no `--randomize-all`) is unchanged.

### R2-6: reporter citations

Verified: the case "both open, nightly success" is `hack/test/ci_failure_report_test.sh:1226-1233`
(`expect` at `:1233`); the concurrency block is `ci-failure-report.yaml:40-42` (`workflow_id` at
`:41`). The cited case covers the close side only; the per-workflow marker filter is
`hack/ci/ci-failure-report.sh:133-140` (`select(.wf == $wf)`). Fix: citations corrected and the
filter cited. Opus's alternative, a new reporter case "another workflow's issue open, this one
fails, a new issue opens", is rejected: the filter is change 7's code, covered by its tests, and
the scenario is about the reporter, not about this change.

### R2-7 to R2-9

- R2-7 verified (Ginkgo refuses `-count=3`, round-1 V1). Fix: the scenario adds "for a `RunSpecs`
  package, `-count=1`, and its Ginkgo seed".
- R2-8 verified (`ci.yaml:800-801`: `pkg: ./internal/aggregate/...`). Fix: TDS-1 scenario and F5a
  use the package directory, the matrix `pkg` without its trailing `/...`.
- R2-9 verified (README `:115` is `kubectl get kinv,ktgt,ksnap -A`). Fix: each comma-separated
  name must be a `shortNames`, `singular` or `plural` entry of a Kollect CRD, else fail (fail
  closed; the quick start names no built-in resource). New case R3b
  (`kubectl get kollectprofiles,pods` fails naming `pods` only).

## Task sizes after the fixes

1.4 grew past 15 mutants, so it splits into 1.4 (retry-decision mutants, 8) and 1.5 (output-shape
mutants, 8); `task-prompt.md` G1 is now 1.1-1.5. 1.2 has 9 cases, 2.2 has 11, 2.5 has 14 mutants,
3.1 has 11 cases.

## Gates

`task spec:validate` exit 0; `task lint:markdown` exit 0 (2026-10-18, after the fixes).

## Verdict

One CRITICAL (R2-2) and four WARNINGs accepted and fixed mechanically in the spec set; one
alternative rejected (R2-6 extra case). No CRITICAL remains. The spec set is ready for the task
loop.
