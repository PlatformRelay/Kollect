## Verdict: CLEAN

## Findings
- [NOTE] The `lint` step that runs the validator has never run in CI on this branch. It comes after the guard's plain step, which stays red on T17 until 3.1, and a failed step skips the steps after it. So the DUA-5 row "`lint` on the PR head runs the validator" is still unproven. This is not a defect in the diff. — `.github/workflows/ci.yaml:589`, `.github/workflows/ci.yaml:597`
  Failure: a CI-only difference (the runner's docker, the ghcr pull, uid 1001) would only show up after 3.1 lands.
  Fix: keep that matrix row `not-run`. Record a green `lint` run on the PR head before closing DUA-5.
  Confidence: 85
- [NOTE] A mixed group branch may inherit `rebaseWhen: behind-base-branch` from its digest upgrade. As I recall Renovate's `generateBranchConfig`, it takes non-merged fields from the first upgrade after sorting by depName. — `renovate.json:165-170`
  Failure: in a `docker images` branch where a digest update sorts first (e.g. `debian` digest + `gcr.io/distroless/static` tag), the branch has `automerge` false but is rebased every hour whenever main moves. That costs extra CI runs; it never merges anything. `evidence/2.4.md:47-48` says every `mon` branch had `conflicted`, but `mon` has no such ordering, and `ctl` only shows digest-only branches.
  Fix: none needed for safety. Optionally add a 2.4 copy with a digest whose depName sorts before a tag change, and record the branch's `rebaseWhen`.
  Confidence: 40
- [NOTE] The state file still shows `consistency_tool_pins_test.sh --self-test` as "pending: runs once in FINAL", and Status is IN PROGRESS. Group closure depends on that run. — `openspec/changes/dependency-update-automation/evidence/2.1.md:99`
  Failure: the 2.2 rule-order claim "each both modes" is not fully evidenced yet.
  Fix: run it and record the result before ticking 2.2.
  Confidence: 90

What I checked, all holding:
- **Workflow:** cron `17 * * * *` plus dispatch; fixed `concurrency` group `renovate`, `cancel-in-progress: false`. Workflow and job are `contents: read` only (`pull-requests: write` dropped).
- **Token:** `token:` is exactly the secret. The token-check step has no `if:` and exits 1 with a message naming the variable when it is empty, so it fails closed under `bash -e`. `RENOVATE_FORCE` is removed. No `github.token` or `GITHUB_TOKEN` string remains in the file.
- **sonar_ko_08:** the updated comment matches `renovate.yaml`.
- **Validator under `set -euo pipefail`:**
  - A yq error aborts the command substitution.
  - A missing workflow exits 1.
  - Two Renovate steps produce two lines, the anchored `=~` fails, and it exits 1.
  - A tag-only image is refused before docker runs.
  - Each file is validated in its own mode, and `|| rc=1` catches both exit 1 and exit 99.
  - `--user` with a non-12021 uid validated in the `mode600` probe.
- **Rule order:** no rule after the image rule sets `schedule`, `automerge` or `groupName` for `ghcr.io/renovatebot/renovate` (`pinned build tools` matches only `custom.regex`; `go toolchain` matches only go/golang). Nothing after the digest rule sets `automerge` or `rebaseWhen`. `go toolchain` is the last `groupName` rule and the node/python rule is last. The helm rule comes after `github actions` and `pinned build tools`.
- **`evidence/2.4.md`:** (a) the image is in its own branch with `automerge` false, and `isScheduledNow` is false for a `* * * * 3` window. (b) Every Helm site is in `renovate/helm` and `renovate/major-helm`, with a negative control. (c) `automerge` is true only on digest-only dockerfile branches and false on the mixed `go toolchain` branch.
- **Bot-bump control:** the guard stays green when a bot bumps the image and the action.

## Could not check
- I could not run anything (read-only Bash). Sensor results are taken from `evidence/2.1.md` and `evidence/2.4.md`, not reproduced.
- I did not read `hack/test/renovate_policy_test.sh` (T1–T16 clause code) or `evidence/0.4.md` / `evidence/0.5.md`; the pin and argv were taken as cited.
- Renovate's `generateBranchConfig` field-merging is from memory, not checked against 44.179.1 source.
- `/tmp/dua-G3/` probe logs are outside the repo and were not read.
- Behaviour on GitHub (the token step on the runner, the image pull as uid 1001, the dashboard, hourly runs) can only be observed after merge.
