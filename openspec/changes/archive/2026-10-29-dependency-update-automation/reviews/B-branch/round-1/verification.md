# Stage B round 1: verification and triage

Branch `origin/main..adccaeae2` (rebased on `e95e8005d`). Reviewer A (all lenses): CONCERNS.
Reviewer B, spec lens: CONCERNS; fitness lens: CONCERNS. No CRITICAL.

Holistic run on the tip (`task check`): four gates red, all from the local worktree environment,
each green after fixing it: `gitleaks-detect` (an ignored `.cache/plugin/social` manifest left by
a local docs build; `gitleaks detect --no-git` green after removing it), the demo-04 kustomize
guard (mise `kubectl` shim with no version; green with `MISE_KUBECTL_VERSION=1.37.0`), and both
OLM/OperatorHub dist guards (`operator-sdk` not installed in this worktree's `bin/`; green after
`make operator-sdk`).

| ID | Finding | Sev | Raised by | Verified | Disposition |
| --- | --- | --- | --- | --- | --- |
| B1 | mixed group branch inherits `rebaseWhen: behind-base-branch` from a first-sorted digest: rebased when behind, never merged; design and docs say only auto-merge PRs are rebased | WARNING | A, B (spec), B (fitness) | yes (evidence/2.4.md; all legs agree it is cost and accuracy, not DUA-7/DUA-8) | fix 5.1: correct design, rule description and docs; keep T11/T12. B (fitness)'s `auto` alternative rejected: Renovate's `getBranchForceRebase` checks rulesets (`checkRulesetsForForceRebase`, read in the installed Renovate source), so `auto` would rebase every bot PR under the strict ruleset |
| B2 | T9 misses a merge through a third-party `uses:` action or a local script | WARNING | A | yes | defer: already in `evidence/1.1.md` Harness gaps; no such workflow exists, and a new workflow is a reviewed diff. Tracked in the hand-off as a known guard limit |
| B3 | `CONTRIBUTING.md:139` allows squash merges; names only changelog-sync as a bypass | NIT | A, B, B | yes | fix 5.4 |
| B4 | devcontainer features would auto-merge if pinned by digest (rule matches every devcontainer dependency) | NOTE | B (spec) | yes (no feature carries a digest today) | defer: matches DUA-7 as written; narrowing to `matchDepTypes: ["image"]` changes the T11 contract and goes to the hand-off as a proposed follow-up |
| B5 | T7 lets `NODE_OPTIONS`, proxy variables and `GITHUB_COM_TOKEN` through; the action forwards them | NOTE | B (spec) | yes | fix 5.2 |
| B6 | `workflow_dispatch` from any branch runs with the bypass token | NOTE | B (spec) | yes | defer: operator action (an environment with a main-only branch policy holding the secret); secrets already reach push workflows. Listed in operator actions |
| B7 | DUA-3 wording omits the token check step's `RENOVATE_TOKEN` | NIT | B (spec) | yes | fix 5.4 |
| B8 | `task check` does not run the validator | NOTE | B (fitness) | yes | fix 5.3 |
| B9 | required `lint` now pulls the Renovate image from ghcr.io on every PR | NOTE | B (fitness) | yes | accept: recorded in design.md risks by 5.1's edit; the digest pin makes a cache possible later |
| B10 | built-in presets resolve inside the image; no guard reads them | NOTE | B (fitness) | yes | fix 5.4: image-bump review step in the maintainer docs |
