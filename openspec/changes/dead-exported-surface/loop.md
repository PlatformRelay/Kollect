# loop.md — dead-exported-surface (spec-loop-opencode run state)

- **Repo:** /Users/kheimel/.treehouse/kollect-79dca7/2/kollect (treehouse pool worktree; primary checkout untouched)
- **Branch:** `fm/kollect-docs-review-followup`, base `3ee21266` (default-branch tip at start); spec-set commit `af6c58f5`.
- **Feature:** openspec change `dead-exported-surface` (Kollect has no `.specify/`; the spec set lives in `openspec/changes/dead-exported-surface/`; `skip_specs: true` — no observable behaviour changes).
- **Reviewers:** fanout (free only — the firstmate brief forbids any `claude/*` leg).
- **Implementer model:** `GLM-5.3`; **review pairing:** `DeepSeek-V4.1-Flash:diff` + `Qwen3.8-Flash-Next:diff`; strong gate legs `Qwen3.8-2.4T-A95B-NVFP4:spec` / `:adversarial` (spec-loop-opencode's substitute for the Claude strong leg).
- **Sources:** `data/kollect-xconsol-final/report.md` §4 Sweep 2 (read at /Users/kheimel/Projects/Tools/firstmate/data/kollect-xconsol-final/report.md); every item re-verified at 3ee21266 before the spec set was written.
- **Budget:** `--budget hours=8`, claude=0 (never a Claude leg).

## Stages

| Stage | State |
| --- | --- |
| P plan+tasks | done — spec set committed `af6c58f5` (proposal.md, tasks.md, .openspec.yaml skip_specs) |
| R spec-set review | pending |
| L task loop | pending |
| B branch review | pending |
| hand-off | pending |

## Fitness functions (inventory at orient)

| Characteristic | Command | Type | Baseline |
| --- | --- | --- | --- |
| Go layering | `.go-arch-lint.yml` via `task arch-lint` | triggered | allow-list |
| Go coverage floor | `task coverage` (internal/, `COVERAGE_MIN=90`) | holistic, per change | ≥90% |
| Static Go analysis | golangci-lint (`make lint`) + Sonar + govulncheck (`task vulncheck`) | holistic | green on main |
| OpenSpec validity | `task spec:validate` (openspec validate --all --strict + config check) | triggered | green |
| Shell hygiene | `task lint:shell` (shellcheck over hack/**) | triggered | no warnings |
| Go unit suite | `task test` (envtest-backed) | holistic | green |

This branch deletes dead Go surface; the Go rows are the load-bearing ones. No allow-list
widening is permitted: if a gate goes red, classify before touching anything.

## Lessons

- kollect has no `.specify/`; the spec set lives in `openspec/changes/<change>/` with
  `skip_specs: true` in `.openspec.yaml` for pure-refactor changes (validation is `task spec:validate`).

## Known red

none

## Test changes

none
