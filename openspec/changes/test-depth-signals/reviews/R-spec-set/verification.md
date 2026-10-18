# R spec-set review: verification

Change `test-depth-signals`, spec set at `2aef4cb1d` (content of `da4009a78`), reviewed 2026-10-18.
Target question: would executing `tasks.md` as written deliver the spec?

## Legs

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (spec lens, cold brief `brief-astra.md`) | `leg-astra-spec.md` | 0 | BLOCK |
| claude/opus:spec (fanout.sh, `--no-unify`) | `leg-claude-opus-spec.md` | 0 (134 s) | CONCERNS |

Both legs ran. No stand-in. The orchestrator verified every CRITICAL and WARNING below against the
tree and, where stated, by running a probe. Every claim is tagged verified (read or run, 2026-10-18)
or believed.

## Findings

| Id | Sev (after verification) | Legs | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| V1 | CRITICAL | astra (R1) + orchestrator | yes | `-count=3` cannot run the two Ginkgo suites | fix |
| V2 | WARNING | astra (R2) + orchestrator | yes | the `-shuffle`/`-count` premise is wrong: one shuffle per binary, the same order every iteration | fix |
| V3 | WARNING | astra (R4), opus | yes, probe | the corpus check is blind in two of three fuzz packages | fix |
| V4 | WARNING | astra (R7), opus | yes | envtest provisioning is only the cache; pipeline envtest tests skip without assets | fix |
| V5 | WARNING | astra (R3) | partly | the retry rule is substring-based, not an allow-list of the flake's lines | fix |
| V6 | WARNING | astra (R5) | yes | F3, F4, F5 cannot kill their 1.3 mutants | fix |
| V7 | WARNING | astra (R6), opus (NOTE) | yes | no case asserts the fuzz argv or the executed shuffle package set | fix |
| V8 | WARNING | opus | yes | 2.2 names no `--self-test` mutants for `test_shuffle_test.sh` | fix |
| V9 | WARNING | astra (R8) | partly | CFC-6 registration is conditional | fix |
| V10 | WARNING | astra (R9) | yes | README guard lacks empty-section, unknown-namespaced-task and out-of-scope controls | fix |
| V11 | WARNING | astra (R10) | partly | S8 omits `setup-task`, `persist-credentials: false`, `timeout-minutes`; no lint runtime row | fix |
| V12 | WARNING | astra (R11), opus (NOTE) | yes | the post-merge row waits for a reporter issue a green nightly never files | fix |
| V13 | WARNING | orchestrator | yes | `go list ./...` holds no e2e package: the exclusion list is empty today | fix |
| V14 | NOTE | opus | yes | no case pins `--- FAIL: <target>` to the matrix target | fix (with V5) |
| V15 | NOTE | opus | yes | "SHALL NOT duplicate `demo_task_aliases_test.sh`" has no check | reject |
| V16 | NOTE | orchestrator | yes | stale or imprecise citations | fix |
| V17 | NOTE | orchestrator | believed | golang/go#75804 is unverified and would be printed in the warning | defer: task 1.1 |

### V1 CRITICAL: `-count=3` cannot run the Ginkgo suites

Verified 2026-10-18. `go.mod:18` pins `github.com/onsi/ginkgo/v2 v2.33.0`. In the module cache,
`types/config.go:402-410` appends `InvalidGoFlagCount()` when `-count`/`test.count` was set to
anything but 1 (`types/errors.go:656`: "Ginkgo does not support using go test -count to rerun
suites. Only -count=1 is allowed."). `core_dsl.go:261-262` also exits on a second `RunSpecs`
(`RerunningSuite`). Both suites call `RunSpecs` (`internal/controller/suite_test.go:51`,
`internal/webhook/v1alpha1/suite_test.go:32`), and both are in scope under decision A. Executing
2.3 as written ships a nightly that is red every night; 2.1 cannot measure a passing wall time.
Not run on envtest here; the source is unambiguous.

Fix (TDS-2, proposal *What Changes* and *Assumptions*, tasks 2.1-2.3): the script runs packages
whose `_test.go` files call `RunSpecs` with `-shuffle=<seed> -count=1` three times (three
invocations), and every other unit package with `-shuffle=<seed> -count=3`; detection is by
grep, not a hard-coded list. Add an S-case: a fake discovery with one Ginkgo package and one plain
package, assert the Ginkgo package never gets `-count` other than 1 and is invoked three times; a
mutant that passes `-count=3` to it turns the case red. 2.1 records one real smoke run of both
suites under the split. Scope stays decision A (no package dropped), so no new operator decision.

### V2 WARNING: the shuffle premise is wrong

Verified in Go 1.27.2 `src/testing/testing.go:2570-2587`: `M.Run` seeds once, prints
`-test.shuffle N`, shuffles `m.tests` once; the `*count` loop at `:2714` reruns that same order.
So `-count=3` gives one permutation repeated three times, not three permutations, and a failure
can reproduce with another `-count` (unless it needs state from an earlier iteration).

Fix: replace proposal *Assumptions* bullet 3 and the TDS-2 "Seed reproducible" parenthesis with
"each test binary shuffles once with seed N and repeats that order; reproduce with the same seed
and repetition settings". In 2.1 replace "reproduces only with the same seed and the same
`-count`" with "reproduces with the same seed and `-count`, and record whether it also reproduces
at `-count=1`".

### V3 WARNING: the corpus check is blind where `testdata/` is untracked

Verified by probe (scratch repo, 2026-10-18): a new
`internal/aggregate/testdata/fuzz/FuzzContentHash/abc` shows in `git status --porcelain` as
`?? internal/aggregate/testdata/`, and `grep 'testdata/fuzz/'` does not match;
`--untracked-files=all` shows the file. `git ls-files` finds no tracked `testdata/` in
`internal/aggregate` or `internal/sink/git` (only `internal/collect/testdata/*.json`).
`.gitignore` has no `fuzz`/`testdata` entry. The deny marker `Failing input written` is the only
backstop, so this is defence in depth, not a fail-open of the whole rule.

Fix (TDS-1, 1.2, 1.3): "`git status --porcelain --untracked-files=all` shows no path under
`testdata/fuzz/`, and a failing `git status` fails closed". F5 splits into F5a (new corpus file in
a package with no tracked `testdata/`, no marker) and F5b (`Failing input written`, no file). 1.3
adds a mutant dropping `--untracked-files=all`.

### V4 WARNING: envtest is cached, not provisioned

Verified: `.github/actions/go-cache/action.yml:31-40` only restores `bin/k8s`;
`ci.yaml:647-649` is that cache step; provisioning is `task coverage` (`ci.yaml:654`) ->
`hack/coverage.sh:23-30` (`make setup-envtest`, export `KUBEBUILDER_ASSETS`, fail when empty).
On a cache miss without that, `internal/pipeline/integration_test.go:45` and `:132` `t.Skip`. (The
Ginkgo suites would fail instead, so the run is not silently green overall, but the L2 tests are
dropped against decision A.) S8 checks only `envtest: true`.

Fix (2.3, S8): `hack/ci/test-shuffle.sh` runs `make setup-envtest`, exports `KUBEBUILDER_ASSETS`
from `make -s echo-kubebuilder-assets` and fails when empty, as `hack/coverage.sh:23-30`; cite that,
not `ci.yaml:647-649`. Add an S-case plus mutant (drop the provisioning, assert non-zero or the
missing-assets message).

### V5 WARNING: the retry rule is a substring match

Partly verified: TDS-1 (`spec.md:12-18`) accepts any output that contains the two lines and none
of four deny markers. Additional unknown diagnostics (for example `fuzzing process hung or
terminated unexpectedly`, not a deny marker) alongside the deadline pair are retried. Truncated
capture is believed unlikely (a kill prints `signal: killed`). Decision A says "retry only the
known deadline-flake text", which an allow-list states more exactly. Downgraded from astra's
CRITICAL: the realistic crash paths print a deny marker.

Fix (TDS-1, 1.1): the failed attempt qualifies only when every line other than `fuzz:` progress
lines matches the flake's line set captured in the fixtures (`--- FAIL: <target> (…)` for the
matrix target, indented `context deadline exceeded`, `FAIL`, `exit status 1`,
`FAIL\t<pkg>\t…`), in that order; anything else is `unrecognised failure text`. Add F12 (deadline
pair plus one unknown line -> one call, `unrecognised`) and F13 (deadline under
`--- FAIL: FuzzOther` -> one call), which also covers V14.

### V6 WARNING: F3, F4, F5 do not isolate their mutants

Verified by reading `tasks.md:38-46,61-63`: F3 and F4 carry only the deny marker, so a mutant
dropping that marker still fails them as unrecognised text and survives; F5 allows "or
`Failing input written`", so a corpus-check mutant survives when F5 uses the marker.

Fix: F3 and F4 are the deadline fixture plus the marker (all else retry-eligible); F5 per V3;
the retry-cap mutant gets a fake with a third (passing) reply and the case asserts exactly two
calls.

### V7 WARNING: no argv or package-set assertion

Verified: F1-F10 assert exit status and call counts; F11 checks the workflow passes the matrix
values. A runner passing `-fuzztime=1s` or another target stays green. S5-S7 check the exclusion
list, not what ran.

Fix: F1 and F10 assert the recorded argv equals `-run=^$ -fuzz=<target> -fuzztime=30s <pkg>`; 1.3
adds a `-fuzztime` mutant. A new S-case with a fake `go list` asserts the executed package set
equals discovery minus exclusions, plus a drop-one-package mutant.

### V8 WARNING: 2.2 lists no self-test mutants

Verified: 1.3 lists the fuzz guard's mutants; 3.1 frames R1-R6 as tree mutants; 2.2 lists cases
only, and the Verification row cites "guard mutants". The stub already passes `-count=3`, so no
case notices it dropped.

Fix: add to 2.2: "`--self-test` mutants, one per defect: drop `-shuffle`, drop `-count=3`, echo
the seed after the run, a reasonless exclusion, a non-e2e exclusion, an empty package list, drop
envtest provisioning (V4), `-count=3` on a Ginkgo package (V1); each turns its case red, a no-op
copy passes."

### V9 WARNING: CFC-6 registration is conditional

Partly verified: `hack/test/ci_workflow_security_test.sh:597-612` explains that CWS-6 discovers
only guards CI already invokes, so a guard whose lint steps are deleted drops out; `cfc_guards`
pins named guards in both modes. `tasks.md:9-10` registers a new guard only "if it checks its own
wiring", and no case says any does. Most existing guards are unregistered, so this is the repo's
current posture, not a fail-open specific to this change.

Fix: in 1.2, 2.3 and 3.2, add each new guard to `cfc_guards` (`:603-604`) and record that
`ci_workflow_security_test.sh` plain and `--self-test` stay green.

### V10 WARNING: README guard controls

Verified by reading 3.1: no case with the heading present and no `sh` fence (empty extraction),
none for an unknown namespaced task (`task test:nosuch`), no green control with an invalid
command in prose, another section or a non-`sh` fence.

Fix: add R8 (section present, no `sh` fence -> fail), R9 (`task test:nosuch` -> fail, names it),
R10 control (bad commands outside the section, in prose and in a `text` fence -> pass).

### V11 WARNING: workflow contract and lint runtime

Partly verified: `ci.yaml:650-652` pins `go-task/setup-task` (needed for `task test:shuffle`);
every workflow checkout uses `persist-credentials: false`. S8 checks neither, nor
`timeout-minutes`. CWS-4 (`ci_workflow_security_test.sh:390`) covers `ci.yaml` and
`e2e-smoke.yaml` only, so concurrency is optional. No task measures what the three guards add
to the required `lint` job (lint ~12 min per change 5's lessons, in `loop.md`).

Fix: S8 also requires pinned `go-task/setup-task`, `persist-credentials: false` and the
`timeout-minutes` measured in 2.1. Add a Verification row: wall time of each new guard, both modes,
and the lint delta, recorded in the group evidence.

### V12 WARNING: post-merge evidence depends on a failure

Verified: `tasks.md` Verification row "reporter issue on a scheduled failure" can never close on
a healthy nightly; 2.3 notes `workflow_dispatch` runs are never reported. The two-issues scenario
is held by design (`ci-failure-report.yaml:38-40`, concurrency per `workflow_id`; one issue per
workflow in change 7's tests).

Fix: the post-merge row becomes "first scheduled `Test shuffle` run completes and a
`CI failure report` run fires for it (`workflow_run`)". The two-issues scenario cites change 7's
existing per-workflow case in `ci_failure_report_test.sh` instead of a live failure. No owner
account is needed.

### V13 WARNING: there is no e2e package to exclude

Verified: `go list ./...` (2026-10-18) lists 47 packages, none matching `e2e`; `test/e2e/` holds
only `fixtures/`. The spec's "the list SHALL hold only the e2e packages" and S6 assume entries
exist; an implementer may add a path that matches nothing.

Fix: TDS-2 and 2.1: "the exclusion list is empty on 2026-10-18 (`go list ./...` has no e2e
package); an entry is allowed only for an e2e package and needs a reason". S5 and S6 stay as
mutants on a copy.

### V14 NOTE: target pinning

Folded into V5 (F13).

### V15 NOTE: no check for "SHALL NOT duplicate"

Reject: the rule is held by review, and a guard asserting the absence of duplicated checks would
cost more than the duplication it prevents. The spec now says "(held by review)" after the rule.

### V16 NOTE: citations

Verified 2026-10-18; all other cited lines match.

| Claim | Cited | Actual |
| --- | --- | --- |
| `fuzz` job | `ci.yaml:817` | job at `:790`; `:817` is the step `Native Go fuzz (30s)` |
| check.sh guard sweep | `hack/check.sh:88-108` | loop `:88-111` |
| envtest provisioned as CI | `ci.yaml:647-649` | cache only; provisioning `hack/coverage.sh:23-30` via `ci.yaml:654` (V4) |
| unit package filter | `hack/coverage.sh:37` | also drops `/internal/` and `/cmd$` (internal at `:43-45`); `Makefile:70` is the right citation |
| CFC-6 list | "there" (CWS-6 `:429`) | `cfc_guards` at `ci_workflow_security_test.sh:603-604` |

Correct: `ci.yaml:279`, `:826-838`, `:828`; `ci-failure-report.yaml:22-24`;
`ci_failure_report_test.sh:1547-1553`, `:424`; `ci_workflow_security_test.sh:429`;
`README.md:106`; `Taskfile.yml:59`; `hack/coverage.sh:23`; `Makefile:70`; the cron table; "no
guard mentions `fuzz`" (`grep -ln fuzz hack/test/*.sh` empty).

### V17 NOTE: upstream issue id

Believed: golang/go#75804 is unverified (no network here). Defer to task 1.1: verify it before it
goes into the warning text, or name the flake without the number.

## Operator items

None. No task needs an operator account; the post-merge row (V12) needs only reading run history.

## Verdict

One accepted CRITICAL (V1), twelve WARNINGs to fix in the spec set before the task loop. Per the
duo table, an accepted CRITICAL earns a second spec-set round after the fix.

## Applied (round 1 fixes)

Applied to the spec set 2026-10-18 (proposal.md, specs/test-depth/spec.md, tasks.md; task-prompt.md
only for the new dispatch groups G1 = 1.1-1.4, G2 = 2.2-2.5). Every fix above is in, with these
notes:

- V1: TDS-2 runs `RunSpecs` packages as three `-shuffle=<seed> -count=1` invocations, the rest
  with `-count=3`; S8 and a 2.5 mutant hold the split; 2.1 smoke-runs both Ginkgo suites.
- V5 and V6 interact: with an allow-list, a deadline fixture plus a deny marker is rejected even
  when the marker check is dropped. So F2-F4 and F5b also assert that the message names the
  marker, and the drop-marker mutants are killed through that message.
- Split for size: 1.1 (fixtures, F1-F6 with F5a-F5c) and 1.2 (F7-F13); 1.3 implements, 1.4
  mutants. 2.2 script cases S1-S10, 2.3 wiring cases S11-S12, 2.4 implements, 2.5 mutants.
- V16 citations corrected and re-checked 2026-10-18: `ci.yaml:790` (job), `:817` (step),
  `:827-838` (loop body; `:826` is `run: |`), `hack/check.sh:88-111`, `hack/coverage.sh:23-30`,
  `Makefile:70`, `ci_workflow_security_test.sh:603-604` (`cfc_guards`), `:597-602` (its comment),
  Go `testing.go:2572-2587` and `:2714`, Ginkgo `core_dsl.go:260-262`; the per-workflow reporter
  case is `ci_failure_report_test.sh:1225-1232`.
- V15 rejected (above). V17 deferred: task 1.1 verifies the id or names the flake without it; the
  spec's scenario now says "a warning naming the deadline flake".
