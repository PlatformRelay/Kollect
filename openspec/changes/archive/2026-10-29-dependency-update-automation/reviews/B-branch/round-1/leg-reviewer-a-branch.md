| ID | Severity | File:line | Finding | Suggested fix |
|---|---|---|---|---|
| B-1 | WARNING | `docs/development/tooling-setup.md:190`; `openspec/changes/dependency-update-automation/design.md:25` | “Only on conflict” is false for some manual mixed-group PRs. The first sorted upgrade supplies `rebaseWhen`; a digest-first group can inherit `behind-base-branch` while the all-upgrades calculation keeps `automerge: false`. This causes extra rebases and CI runs, contradicting the design and maintainer guidance. | Qualify both statements with the mixed-group exception and correct the “cost only; no requirement” triage. This preserves T11/T12 unchanged. |
| B-2 | WARNING | `hack/test/renovate_policy_test.sh:457` | T9 does not enforce DUA-8’s whole “no merging workflow” promise. A workflow calling a third-party merge action through `uses:`, or invoking a shell script containing the merge command, escapes its `run`/`with.script` scan. This is a documented coverage gap, not an unsafe workflow currently present. | Cover merge-capable actions and referenced local scripts, with mutants requiring T9’s own diagnostic; otherwise explicitly narrow the claimed guard guarantee. |
| B-3 | NIT | `CONTRIBUTING.md:139` | Squash merges are described as allowed, contrary to the recorded rebase-only ruleset. This predates the branch and is outside its implementation scope. | Track a separate documentation correction removing the squash exception. |

| Requirement | Compliance | Evidence / qualification |
|---|---|---|
| DUA-1 | Holds | `.github/workflows/renovate.yaml:23` grants read-only permissions; `:31` checks the secret before execution; `:48` supplies only `secrets.RENOVATE_TOKEN`. T1–T3/T7 cover the specified regressions. Hosted-App guidance is at `docs/development/tooling-setup.md:203`. |
| DUA-2 | Stronger than spec | `.github/workflows/renovate.yaml:12` runs hourly; `:18` fixes concurrency without cancellation. The requirement asks for at least daily. |
| DUA-3 | Holds | `.github/workflows/renovate.yaml:44` pins the action; `:47` pins the image. `renovate.json:151` isolates the image with Monday scheduling, `updateNotScheduled: false`, and no automerge. T6/T7/T14 check these. |
| DUA-4 | Holds | `renovate.json:3` enables the recommended preset without disabling the dashboard; `hack/test/renovate_policy_test.sh:544` rejects disabling settings and nested presets. Actual dashboard creation remains post-merge verification. |
| DUA-5 | Holds | `hack/ci/renovate-config-validate.sh:32` derives and validates the pin; `:44` validates both modes and propagates failures. `.github/workflows/ci.yaml:598` invokes it unconditionally. Exact-head CI execution is still recorded as unverified. |
| DUA-7 | Holds | `renovate.json:164` contains the sole digest-only automerge rule. `hack/test/renovate_policy_test.sh:569` constrains its location and exact keys. `evidence/2.4.md:52` and `:54` demonstrate mixed branches remaining manual; `:78` demonstrates eligible digest-only branches. |
| DUA-8 | Partial | Runtime configuration holds at `renovate.json:4` and `:169`; pinned-source evidence at `evidence/0.4.md:191` supports status checking, rebasing and rebase merge. The no-merge-workflow regression guarantee is incomplete: B-2. |
| DUA-9 | Holds | `renovate.json:193` matches `helm/helm` by package name. `evidence/2.4.md:93` records all seven sites together in both minor and major branches. |
| DUA-10 | Partial | The requested maintainer topics are present at `docs/development/tooling-setup.md:159`. The rebase guidance at `:190` contradicts the known mixed-group behavior: B-1. |

The mixed-group note does **not** identify a DUA-7 merge-scope breach or a violation of DUA-8’s explicit configuration requirements. It does invalidate the design’s exclusivity claim and the corresponding maintainer guidance. The smallest warranted fix is documentation, retaining the current T11/T12 contract. A configuration change intended to prevent these rebases would need to preserve digest-only eligibility and be tested against both mixed and digest-only groups; simply overriding the Go group’s `rebaseWhen` would violate T12 and weaken the intended digest rebase policy.

The requested unre-reviewed fixes check out statically:

- T10 recursively rejects nested presets; T7 covers workflow/job/step environments and parsed `GITHUB_ENV` references.
- T14 requires `updateNotScheduled: false` and a whole-weekday window. T13 correctly has a green baseline with negative mutants.
- The lookup evidence contains actual mixed upgrades, plus digest-only controls. Hourly scheduling is implemented; its claimed restriction of rebases is qualified by B-1.
- T6 now requires exactly `.github/renovate-config.json` at `hack/test/renovate_policy_test.sh:374`, with a corresponding mutant. The shell-resolution and failed-read fixes are present.
- T16’s post-round-2 wording at `:819` accurately explains an unrecognized value-taking Docker option.
- The self-test requires each mutant’s own diagnostic and detects incomplete background cases. Recorded clause-removal results support that the clauses are load-bearing.

Architecturally, this branch expands the **required CI gate set**, improves **supply-chain pinning**, and delegates narrow **merge authority** to Renovate. T8/T16 and CWS-6 guard validation wiring; T6/T7/T14 guard execution configuration; T10–T13 guard merge scope and settings. These are useful structural fitness functions, with the documented lexical and rule-resolution limits. Additional controls beyond explicit requirements include the validator’s network isolation/read-only mount and exact action-input/global-config allowlists.

Review was read-only. Shell syntax checks passed. Runtime and mutation conclusions rely on the recorded evidence; I did not rerun mutants or the full local gate.

## Verdict

CONCERNS
