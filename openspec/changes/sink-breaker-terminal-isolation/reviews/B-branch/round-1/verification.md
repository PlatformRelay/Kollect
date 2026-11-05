# B round 1 — verification and triage

Branch tip `4e50a9a2f` (rebased on `origin/main` after the Helm v4 and mongo-driver v2 merges).
Reviewer A (adversarial): CONCERNS. Reviewer B (spec): CONCERNS. Both: SCB-1, SCB-2, SCB-3 hold. No
CRITICAL. Each finding below was checked against the code by the orchestrator on 2026-11-05.

| # | Finding | Sev | Reviewers | Verified | Disposition |
|---|---|---|---|---|---|
| 1 | Only one of the routed raise sites (`foreignClaimError`) is pinned by a test; reverting any other to `invalidPrune` keeps every test green | WARNING | A, B | yes: no `IsInventoryScoped` assertion in `internal/sink/git` tests; G1's site mutation covers one site | fix: task B1.1 (per-site table + per-site mutations) |
| 2 | SCB-1e drives only the CLI engine (`file://` remote); the go-git engine's return path is untested | WARNING | B | yes: `isFileRemote` routes the fixture to `exportViaCLI` | fix: B1.1 go-git case; design *Risks* narrowed |
| 3 | tasks.md test-rule race command fails on Ginkgo `-count>1` | WARNING | A | yes (`evidence/G1.md`) | fix: tasks.md test rules |
| 4 | "Green at every commit" stronger than evidence (inherited flakes until G2) | NOTE | A | yes | fix: tasks.md preamble |
| 5 | T0.1 says cleanup bypasses the breaker; relational cleanup calls `RunExportEnvelope` (`cleanup.go:379`) | NOTE | A | yes | fix: B1.1 evidence wording |
| 6 | `InventoryScoped` always yields Terminal, stronger than "keeps the class" | NOTE | B | yes (`errors.go:156-162`) | fix: SCB-1 text "make the error Terminal"; doc comment in B1.1 |
| 7 | `ErrTooManyRequests` passes the Git classifier for every sink type and logs "git transport: too many requests" | NOTE | B | yes (`export.go:208`, `git/errors.go:45`) | defer: pre-existing, class unaffected; backlog SINK-BREAKER-TOOMANY-MSG; design.md corrected |
| 8 | Some own-path rejections (`prune_owned.go:53,57`, `git/export.go:95`) still count | NOTE | B | yes; no reachable input found | reject: the spec's fallback (undecided sites stay unmarked, T0.1) |
| 9 | A Terminal-marked trial can keep the breaker half-open indefinitely | NOTE | A, B | yes | reject: accepted consequence, recorded in proposal *Impact*; each excluded trial frees the slot, so no lock-out |

Holistic `task check` on `4e50a9a2f`: exit 1, four gates red, none from this branch — `gitleaks-detect`
(one hit in the git-ignored mkdocs cache `.cache/plugin/social/manifest.json` left by a local strict
docs build; removed), `demo_04_samples_kustomize_test` (mise `kubectl` shim without a version),
`dist_olm_bundle_test` and `dist_operatorhub_pr_test` (no local `operator-sdk`). The last three fail
identically on a clean `origin/main` worktree (checked 2026-11-05). All other gates green.
