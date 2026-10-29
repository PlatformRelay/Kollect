## Verdict: BLOCK

## Findings
- [CRITICAL] The DUA-7/DUA-8 guard only checks `renovate.json`, so there are several ways to widen auto-merge under the bypass token that it would not catch. `tasks.md:24-25`
  Failure: T11 and T12 read only `packageRules`, the top-level `automerge` and `extends` names in `renovate.json`. Each of these mutants passes and still turns on auto-merge or platform auto-merge:
  - `"vulnerabilityAlerts":{"automerge":true}`, `"lockFileMaintenance":{"automerge":true}`, `"gomod":{"automerge":true}`, `"minor":{"automerge":true}`
  - a `github>x/preset` in `extends`, because only preset names containing "automerge" are refused
  - a `packageRules` entry or `"platformAutomerge":true` in `.github/renovate-config.json` (the global config, `renovate.yaml:33`)
  - `RENOVATE_FORCE={"automerge":true}` or `RENOVATE_PLATFORM_AUTOMERGE`/`RENOVATE_IGNORE_TESTS` env in `renovate.yaml` (`RENOVATE_FORCE` already exists, `renovate.yaml:24`)

  Fix:
  - T11: count `[.. | objects | select(has("automerge"))]` across both config files; it must be exactly 1 and sit inside the allowed rule.
  - Allow-list `extends` exactly (`config:recommended`).
  - Give the global file a fixed key set.
  - Refuse any `RENOVATE_*` env in `renovate.yaml` other than `RENOVATE_REPOSITORIES` and today's exact `RENOVATE_FORCE` expression.

  Confidence: 85
- [WARNING] No task adds the validator step to `ci.yaml`'s `lint` job, so T9 can never turn green as tasks are written. `tasks.md:30,36`
  Failure: 1.1 wires only the guard's two modes; 2.1 edits `renovate.yaml`, 2.2 `renovate.json`, 2.3 the script body. DUA-5 (`spec.md:88`) stays absent unless the implementer goes beyond the tasks.
  Fix: in 2.3, add "add the step `run: bash hack/ci/renovate-config-validate.sh` to `lint` (no `if:`, no `continue-on-error`)".
  Confidence: 80
- [WARNING] T13 as specified ("runs `gh pr merge` at all") is red on today's tree, which contradicts the claim that T13 is green today. `tasks.md:26,30`; `.github/workflows/ci.yaml:13`
  Failure: a grep over workflow text hits the comment at `ci.yaml:13`, so 1.1's recorded baseline is wrong, or the implementer quietly loosens the check.
  Fix: T13 matches only `run:` bodies parsed with yq (comments ignored), and also matches `pulls/*/merge`, `--auto` and `enablePullRequestAutoMerge`.
  Confidence: 75
- [WARNING] T11 leaves `automerge` itself out of the rule's allowed key set. `tasks.md:24`
  Failure: implemented literally ("keys are exactly matchManagers + matchUpdateTypes + optional description"), the correct rule `{matchManagers, matchUpdateTypes, automerge}` fails, so the test does not pass on a correct tree.
  Fix: allowed keys are `{automerge, matchManagers, matchUpdateTypes}` ∪ optional `description`.
  Confidence: 70
- [WARNING] T7 and T14 don't check rule order, and in Renovate a later `groupName` wins. `tasks.md:20,27`; `renovate.json` "github actions" and "pinned build tools" rules
  Failure:
  - If the Renovate-image rule sits before the `github-actions` group rule (`groupName: "github actions"`), the image joins that group. T7 still passes, but DUA-3's "own PR" is broken.
  - If the helm rule moves above "pinned build tools", the `custom.regex` pins leave `renovate/helm`. T14 still passes.

  Only the post-hoc 2.4 lookup would notice.
  Fix: assert index(image rule) > index("github actions") and index(helm) > index("pinned build tools") and index("github actions"), with a mutant for each. This mirrors `consistency_go_toolchain_test.sh:256-271`.
  Confidence: 75
- [WARNING] T2 only exercises the step's `run:` body, so a fail-closed step can be defeated around it. `tasks.md:15`
  Failure: these pass T2 but don't fail closed: `if: false`, `continue-on-error: true`, or `env: RENOVATE_TOKEN: ${{ secrets.OTHER }}` (setting the variable by hand makes the body pass).
  Fix: T2 also requires that the step has no `if:`/`continue-on-error` and that its env maps `RENOVATE_TOKEN` to exactly `${{ secrets.RENOVATE_TOKEN }}`.
  Confidence: 70
- [WARNING] T10 requires a single validator call on both files, which may validate them in the wrong mode. `tasks.md:23`; `spec.md:90-91`
  Failure: when files are named on the command line, `renovate-config-validator` validates them as global config unless `--no-global` is passed (from memory of 44.x, not verified). One call either validates `renovate.json` too leniently (as global) or fails `.github/renovate-config.json` on its global-only `onboarding`/`requireConfig`. "Exactly once" forbids the correct two calls, and the fixed argv order conflicts with an `--entrypoint` form that 0.5 may find is needed.
  Fix: have 0.5 record the mode behaviour and the entrypoint; write T10 against two calls (repo mode for `renovate.json`, global mode for the other file) and argv sets, not a fixed sequence.
  Confidence: 55
- [WARNING] No task probes the most important assumption: that Renovate treats "no checks" as pending. `proposal.md:87-91`
  Failure: the proposal says "Probe: task 2.1", but 2.1 is the workflow edit and 0.4(a-d) doesn't cover it. This is the residual risk named in `design.md:46-48` (a bypass merge when no check started); it is first observed post-merge, on a real digest PR.
  Fix: add a 0.4(e) that reads Renovate's GitHub `getBranchStatus` at the pinned tag (zero statuses and zero check runs gives `yellow`), with evidence.
  Confidence: 65
- [NOTE] Stale task cross-references. `proposal.md:91,98,100`; `design.md:72`
  The probes are cited as tasks 2.1/2.2 (and the lookup as "task 2.1"), but they are tasks 0.4/0.5.
  Confidence: 95
- [NOTE] Manual dispatch makes the Renovate image bump ignore its weekly schedule. `renovate.yaml:24`
  `RENOVATE_FORCE {"schedule":null}` on dispatch overrides the weekly rule, so DUA-3's "at most weekly" (`spec.md:68`) doesn't hold for manual runs. Fix: document this, or force only `"schedule":null` for the non-image rules.
  Confidence: 60
- [NOTE] T4 and T3 have small gaps. `tasks.md:16-17`
  - T4 ignores the month field, so `0 4 * 1 *` passes; it also rejects a daily `0-6` day-of-week. Fix: require `*` in month too.
  - T3 should also match the string form `permissions: write-all`.

  Confidence: 60
- [NOTE] T6 takes the action's version from its trailing comment, so an older SHA labelled `# v46.3.10` passes. `tasks.md:19`
  The 0.4(d) evidence should record the SHA for v46.3.10, and T6 should pin that exact SHA.
  Confidence: 50
- [NOTE] Some verification rows are mislabelled or missing. `tasks.md:56`
  - The DUA-3 "run log shows the digest reference" row is marked not-run, but a run log exists only after merge; it should be post-merge.
  - DUA-3 "image PR not auto-merged" (`spec.md:65-68`) has no row.
  - DUA-8 "behind PR is rebased and re-tested" (`spec.md:142-145`) has no row.
- [NOTE] The rewritten header comment in 2.1 must not contain the literal `github.token`, or T1 fails on a correct tree. `tasks.md:14,34`

Checked and fine:
- 2.2 keeps "go toolchain" as the last group rule and the node/python rule last (`consistency_go_toolchain_test.sh:256-287`).
- A digest-only golang branch in "go toolchain" auto-merges as intended.
- The `lint` job has no job-level `if:` (`ci.yaml:283-286`).
- Adding the guard to the CFC-6 enrolled list makes both modes mandatory (`ci_workflow_security_test.sh:614-629`).
- The task order 0 → 1 (red) → 2 → 3 is sound.

## Could not check
- Renovate 44.x source (validator global/repo mode, `getBranchStatus`, branch-level automerge across mixed upgrades) and `renovatebot/github-action` v46.3.10 `src/docker.ts`. These are judged from memory of upstream, not read.
- `docs/development/tooling-setup.md`, `docs/security/sca-remediation-policy.md`, `hack/test/sonar_ko_08_workflow_permissions_test.sh`, `hack/check.sh` guard-sweep details, `loop.md`, the untracked `reviews/` dir and the referenced guard-mutation checklist (outside the repo).
- Live ruleset 18802410 and run log 39230872281. No tests or guards were run.
