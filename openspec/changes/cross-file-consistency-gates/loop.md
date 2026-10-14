# Loop — cross-file-consistency-gates (spec-loop run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/cross-file-consistency-gates` · Branch: `feat/cross-file-consistency-gates` · Base: `origin/main` @ `0c1456b5d` · Started: 2026-10-14 · Reviewers: codex (`codex exec --sandbox read-only`) + `claude/sonnet` via `fanout.sh --no-unify` (no opencode on this host); `claude/opus` at gates
Next: L — dispatch task 1.1
Budget: claude review legs 0/12 (B reserve 3) · active hours 0/8 (sessions: 2026-10-14–) · source: default

OpenSpec repo (no `.specify/`): the feature dir is `openspec/changes/cross-file-consistency-gates/`
(proposal.md, specs/cross-file-consistency/spec.md, tasks.md). Landing order position 4 of 10;
1–3 merged (#450, #452, #453). Issue #441.

## Stages
- [x] 0 orient — proposal / spec delta / tasks present, nothing implemented (all unticked); branch and base recorded
- [x] P plan + tasks — skipped: present · spec-set fix committed with this file (see Triage)
- [x] R spec-set review — done by other means before #437 merged: commits `a0aba9880` (independent Opus review), `10bc634ed` (round two), `50580a3ad` (cold adversarial), `1745b60ba` (post-merge status). Not repeated. Re-measure on 2026-10-14 found 3 WARNINGs (Triage), no CRITICAL; mechanical fixes applied
- [ ] L task loop — see *Tasks*
- [ ] B branch review
- [ ] Hand-off

## Fitness functions
| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| CI guard meta-tests | `bash hack/test/<guard>_test.sh` and `--self-test` (lint job) | triggered | green suite | per task (touched + new guards) |
| Shell hygiene | `task lint:shell` | triggered | no warnings | per task |
| Workflow supply chain | zizmor gate via `hack/test/ci_workflow_security_test.sh` | triggered | 0 high findings | per task touching `.github/` |
| Go build/vet | `go build ./... && go vet ./...` | triggered | clean | per task touching Go/go.mod |
| Lint incl. arch | `task lint` (golangci v2.13.1 + go-arch-lint) | triggered | green | task 1.3 and at B |
| OpenSpec strict | `task spec:validate` | triggered | 13/13 | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |
| Coverage floor | `task coverage` (COVERAGE_MIN 90) | holistic | ≥90% | at B (Go bump) |
| Vulns | `task vulncheck` | holistic | recorded at 1.3 | task 1.3, at B |

Holistic run at B: pending

## Triage
| Stage | Finding (one line) | Sev | Models | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R | proposal quotes stale versions (1.26.6 / 1.27.1); now go.mod 1.26.9, Dockerfiles 1.27.2 | WARNING | re-measure | fix | proposal.md updated | no |
| R | `.devcontainer/devcontainer.json` pins `golang:1.26.9`; CFC-1 scanned only Dockerfiles, so it drifts after the bump | WARNING | re-measure | fix | CFC-1 + scenario + tasks 1.2/1.3 include the devcontainer image | no |
| R | `lint` is not a `protect-main` required check (required: preflight, test, kind-smoke, Analyze (Go)); CFC-6 "required job" depends on change 1's operator step 5.1 | WARNING | re-measure | defer | owner task (ruleset edit); 6.1 closes with that gap | yes (operator) |

## Tasks
| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|

## Known red
| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes
| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons
- (none yet)

## Where a human should look first

## Proposed harness changes
| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)
| Task | Command sheet |
|---|---|
| ruleset | add `lint` (and per change 1: `vulncheck`, `workflow-security`, `dependency-review`) to `protect-main` required checks |
| 6.2 | merge with the `post-merge` row open; archive in a follow-up evidence PR after the first scheduled `go-patch-lag` run |
