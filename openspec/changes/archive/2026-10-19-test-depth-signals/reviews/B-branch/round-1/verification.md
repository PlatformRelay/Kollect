# Stage B round 1: verification of the branch review findings

Legs: `leg-astra-adversarial.md` (brief: `brief-astra.md`), verdict **BLOCK**;
`leg-claude-opus-spec.md` (spec compliance), verdict **CONCERNS**. The orchestrator checked every
CRITICAL and WARNING below against the code at `548d0d973`, using probes on throwaway copies of the
tree (`git archive HEAD`) and a fake `go` replaying the committed fixtures. Nothing in the tree was
changed.

## Deferred to stage B by the task loop (carried, not re-found)

| Item | From | State at B |
|---|---|---|
| F16 ignores `if:` and `continue-on-error:` on the download step | 1.6 | open; F11 (runner step) and S11 (shuffle job and step) have the same gap, see A2 |
| `make setup-envtest` failure exits without an `::error::` line (`hack/ci/test-shuffle.sh:46`, `set -e`) | G2 | open; the seed line still precedes it in the log (astra Q6) |
| No empty-pkg F0 case and mutant; no case replaying `crash-local.txt` | G1 | open; astra replayed the crash fixture by hand: rejected without retry |
| README guard round-2 fixes (backslash continuation, indented fence) unreviewed | G3 | reviewed now: astra found both present and right for LF input (Q4); opus O1 is about the indented-fence rule's scope |
| `internal: true` tasks count as existing | G3 | open; no internal task named in the README today |

## Findings

| Id | Sev (leg) | Verified | Where | What the code does | Proposed disposition |
|---|---|---|---|---|---|
| A1 | CRITICAL (astra), WARNING by our reading | yes: README line 115 changed to `kubectl get kinv/a foo/b -A` passes the plain guard (exit 0); `kubectl get kinv,foo -A` control fails | `hack/test/readme_quickstart_test.sh:168` | only the first operand after `get` is read as the resource; later `TYPE/NAME` operands are never checked | fix: check the type part of every operand containing `/`, and refuse a second plain operand once a `TYPE/NAME` form is seen; add a case and a mutant |
| A2 | CRITICAL (astra), WARNING by our reading | yes: `continue-on-error: true` on the `task test:shuffle` step passes the plain guard; F11 (`hack/test/fuzz_retry_test.sh:307-331`) has the same gap for the runner step | `hack/test/test_shuffle_test.sh:377-418` (S11) | S11 pins triggers, pins and step order but never reads `if:` or `continue-on-error:` on the job or the steps | fix with the deferred F16 item in one task: S11, F11 and F16 refuse `if:` and `continue-on-error:` on the job and on their steps (precedent: `dist_operatorhub_pr_test.sh:154`), one mutant each |
| A3 | WARNING (astra) | yes: fake `go` printing `deadline-FuzzContentHash.txt` with exit 2 or 137, then the pass fixture: runner exit 0 after 2 calls | `hack/ci/fuzz-retry.sh:67`, `:88` | any non-zero exit with the exact flake text is retried; the spec names only the text | fix: retry only when `rc` is 1 (the status `go test` gives a failed test); cases for 2 and 137 |
| A4 | WARNING (astra) | yes: mutant `\|\| rc=1` to `\|\| true` on the Ginkgo call (`hack/ci/test-shuffle.sh:78`) passes the plain guard | `hack/test/test_shuffle_test.sh` (fake go, S8) | the fake `go` always exits 0 for non-forwarded calls; no case makes a Ginkgo invocation fail | fix: per-invocation exit status in the fake, a case where run 1 of 3 fails and runs 2-3 pass, and that mutant |
| O1 | WARNING (opus) | yes: a `~~~yaml` fence inside the section fails `extract`; the same block in backticks passes | `hack/test/readme_quickstart_test.sh:114` | an indented or `~~~` fence of any language in the section fails; TDS-3 says other-language fences are not read | amend TDS-3: the section allows only column-0 backtick fences (an indented or `~~~` `sh` fence would hide commands). Keep the code |
| O2 | NOTE (opus) | not needed | `readme_quickstart_test.sh:111` | an unclosed non-`sh` fence fails | spec wording at hand-off (fold into the O1 amendment) |
| O3 | NOTE (opus) | not needed | `readme_quickstart_test.sh:158-161` | a flag before the kubectl verb fails | spec wording (fold into O1) |
| O4 | NOTE (opus) | not needed | `readme_quickstart_test.sh:144-149` | `task x -- bar` reads `bar` as a task (false red); `aliases:`, `flatten:` and `dir:` includes are not understood | note; stop at `--` if touched |
| O5 | NOTE (opus) | not needed | `hack/ci/test-shuffle.sh:39` | `SHUFFLE_SEED` must be 1-18 unsigned digits; the script's own seeds are 4-byte | note |
| O6 | NOTE (opus) | not needed | `hack/ci/fuzz-retry.sh:32`, `:49` | target name validated; the `FAIL\t<import path>` line is matched by shape, not against `<pkg>` | note |
| A-Q4 | NOTE (astra, answers only) | not needed | README guard | passes `task -d=/x demo-up`, `kubectl apply -k README.md` (a file); a second `Quick start` section is ignored; CRLF gives a false red | note; the guard proves names exist, not that the quick start runs |

## Compliance (opus spec leg, confirmed)

| Requirement | Leg | After verification |
|---|---|---|
| TDS-1 | holds | holds; A3 is a fail-open path outside the spec's text-only wording |
| TDS-2 | holds | holds for the script (`task test:shuffle` green, seed first, 45 + 2 x 3 runs); its guard misses A2 and A4 |
| TDS-3 | holds, stronger than spec on fences (O1-O3) | partial: A1 multi-operand `get`; O1 needs a spec amendment |

No row is contradicted or absent. Not in any requirement: `workflow_dispatch` on `test-shuffle.yaml`
(unreported by design), the `go mod download` step (F16), and the two counter-increment test fixes.
Astra and opus both read those fixes as real fixes that keep exact assertions (increments 1 and
1024).

Round verdict: **CONCERNS**. There is no CRITICAL at our severity. Four WARNING fixes (A1-A4) and
one spec amendment (O1) are proposed for round 2.
