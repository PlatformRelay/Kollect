## Verdict: CONCERNS

## Findings
- [WARNING] CFR-1's prose contradicts its own decision table. The prose says "failure class and no bot issue … open → SHALL open one", but the table says closed + equal/older key → no write. — `openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md:16-17` vs `:32`, `:124-127`
  Failure: the newest bot issue is closed with key (N+1,1) and the run acted on is (N,2) `failure`. The prose requires an issue and the table and the CFR-3 scenario forbid one. An implementer or reviewer who follows the prose reopens the stale-state bug that round-1 R1 closed.
  Fix: replace the sentence with "…the reporter SHALL take the action its row in the table names", or add "and the table row says open".
  Confidence: 85
- [WARNING] The 1.1 author-spelling probe (the R10 fix) targets an author that is probably absent from this repo. Renovate is self-hosted and authenticates with `RENOVATE_TOKEN` (a PAT) or falls back to `github.token`. Its Dependency Dashboard is then authored by a user or by `github-actions`, not by `renovate[bot]`. — `tasks.md:39-43`, `.github/workflows/renovate.yaml:30-33`
  Failure: `app/renovate`, `renovate[bot]` and `--app renovate` all return `[]`. The probe records no working spelling, and the fake `gh` gets pinned to an argv that was never seen to work against the real API. That is the failure R10 was meant to prevent.
  Fix: 1.1 first reads the dashboard's `author` object, then tests the spellings that follow from it, including `app/github-actions`. If no `github-actions`-authored issue exists, record that and make the lookup argv part of the post-merge row.
  Confidence: 75
- [WARNING] CFR-4 does not require the lookup to be complete. With `gh issue list --state all` at its default `--limit 30`, the per-workflow marker filter runs on a truncated list. The issue comments list has the same gap. — `spec.md:146-152`, `tasks.md:120-121`
  Failure: changes 8/9 add noisy workflows (shuffle, mutation) that open and close many bot issues. Once 30 newer bot issues exist, Go patch lag's long-open issue is no longer in the list. The lookup reports "none", so the next red Monday opens a second open issue (breaking CFR-1 "SHALL NOT be two open issues"), and green never closes the first one.
  Fix: CFR-4 requires a complete lookup: paginate, or exit non-zero when the result count reaches the limit. 3.2 adds a case with ≥31 bot issues from another workflow that are newer than the target's open one.
  Confidence: 65
- [NOTE] The spec never defines a "started job". The failure/no-op split for `cancelled` depends on it, and 1.1 may fall back to "the documented shape". — `spec.md:28,38,55-58`, `tasks.md:34-35`
  Failure: a builder that counts any job with conclusion `cancelled` as started classifies a replaced pending run as a failure and opens an issue.
  Fix: define it, e.g. "a job with non-null `startedAt` whose conclusion is not `skipped`", and pin it in the 3.1 fixtures.
  Confidence: 55
- [NOTE] Running `gh label create ci-failure --force` with no `--color` gives the label a random colour, and `--force` updates the existing label with it on every run. The scenario's "changes nothing the reporter relies on" is true, but the label changes colour on every report. — `spec.md:19-20,48`
  Fix: pass a fixed `--color` and `--description`, and have 4.1 assert them.
  Confidence: 55
- [NOTE] No 2.1 case checks that truncation keeps the excerpt fence closed. A cut inside an excerpt leaves an open fence, so the truncation marker and any text after it render as code. — `tasks.md:59-61`
  Fix: add an assertion to the cap case that the fence is closed before the truncation marker.
  Confidence: 45

Round-1 closure, checked against spec/proposal/tasks:
- **Closed:** R1 (key in the marker, newest issue open or closed), R2 (`gh run list` latest completed), R3 (equal-key rows plus close retry), R4 (fence/`@`/first-line marker), R5 (`workflow=<path>`, both-open case and mutant), R6 (`spec.md:234`, matches `codeql.yaml:28-31`), R7, R8 (table plus parameterised 3.1), R9, R11, R12, R13, R14, R15, R16.
- **R7 shape checked against the CWS-2 awk:** the proposed zizmor block in `tasks.md:167-175` passes `ci_workflow_security_test.sh:257-302`. The two-line comment sets `prev`, the bare `ignore:` is exempt, and the entry has its own comment. Each comment-removal mutant reds.
- **R10 open in substance:** see the second finding.

Cited lines that hold in the tree:
- `e2e-nightly.yaml:2,6,18,127-131,220` and `go-patch-lag.yaml:6,10,17`
- `zizmor.yml:10-14`
- `ci_workflow_security_test.sh:236,303-308,306,459,602-603`
- `hack/check.sh:79,93` and `ci.yaml:279`
- `ci_fetch_lib_hardening_test.sh:865-882`

The nightly's `cancel-in-progress` is false on main (`e2e-nightly.yaml:20`), so the "dispatch replaced a pending run" scenario is the realistic way a run ends `cancelled` without starting.

## Could not check
- The round-2 leg files and `legs.log` in `reviews/R-spec-set/round-2/`: they hold other reviewers' output, so I did not read them.
- Live GitHub behaviour: `gh issue list` default limit, comment pagination, `gh label create --force` recolouring, the Dependency Dashboard's actual author, the `startup_failure` → `workflow_run` delivery. These are inferred, not probed.
- `dist_ci_wiring_test.sh`'s lint-step allowlist and its step-index assertions against the future lint steps. 5.1 delegates this check.
- No implementation exists yet (spec-set stage), so nothing was checked against code.
