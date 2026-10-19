# Stage R round 1 — verification

Reviewed revision: `1c880b52f` (spec set `cb93d3750`). Legs: astra spec lens
(`leg-astra-spec.md`, verdict BLOCK), claude/opus:spec (`leg-claude-opus-spec.md`, verdict
CONCERNS). Each finding was checked against the tree by the orchestrator on 2026-10-19; fixes are
in the spec-set commit that carries this file.

| Id | Sev (leg) | Verified? | Disposition | Where / why |
|---|---|---|---|---|
| A-R1 | CRITICAL (astra) | partly | fix in part, downgraded to WARNING | 0.2 evidence now records opaque checkout labels and pass/fail only, no paths or machine names; a machine-specific phrase inherited from change 9's prompt removed from `task-prompt.md`. Rejected: rewriting the absolute worktree paths in `loop.md`/`task-prompt.md` (every earlier change commits the same run-state layout, and none of it matches the scrub list or names a host) |
| A-R2 | CRITICAL (astra) | yes | fix | 0.2: bounded inventory (kollect clones on this host and their worktrees), no root `AGENTS.md` whatever its header, per-checkout checksums, no overwrite of an existing `AGENTS.local.md`, gate on checkout/rebase/reset/pull of any commit tracking `AGENTS.md`, not only shared `main` |
| A-R3 | WARNING (astra) | yes (lines 6 and 10 contain `.`) | fix | PAC-4 regex-semantics scenario; 1.1 dot-substitution witnesses checked with `rg -i` before use; fixed-string (`-F`) implementation mutant |
| A-R4 | CRITICAL (astra) | yes (`hack/scrub.sh:14` reads disk content of staged paths) | fix | PAC-4 covers working tree and index; 1.1 staged-differs mutants for a scrub string and a private header |
| A-R5 | CRITICAL (astra) | yes (scratch repo 2026-10-19: plain `git check-ignore` does not report a tracked `AGENTS.md`; `--no-index` does) | fix | PAC-4 and 1.1 use `git check-ignore --no-index`, child-path probes, one mutant per path incl. `.agents/`, `.cursor/`, `CLAUDE.local.md` |
| A-R6 | WARNING (astra) | yes | fix | PAC-6: case-insensitive whole-word match, `AGENT_CONTRACT_HOSTNAME` override, skip rules incl. unreadable name, stated as supplementary |
| A-R7 / O-5 | WARNING (both) | yes | fix | PAC-6 grammar: DNS-label end, `cluster.local`/`localhost`/`mise.local.toml`/`git@github.com:` allowed, valid octets, full `172.16.0.0/12`; allowed-look-alikes scenario and green fixture; range-edge mutants |
| A-R8 | WARNING (astra) | yes | fix | `CLAUDE.md` gitleaks mutant; metric-entry mutants; fake-`rg` exit-2 and missing-`rg` mutants replace `chmod 000`; per-mutant expected diagnostic (PAC-5 wrong-clause scenario, task-prompt wording); copy recipe = every tracked file |
| A-R9 | WARNING (astra) | yes | fix in part | PAC-1 sources widened to tracked code; 2.2 cites the tracked source per line and is written by a fresh subagent with only `task-prompt.md`. Rejected: a synthetic OpenSpec preservation test (the workflow runs only `validate`, `list` and `archive`, believed not to write root files); 4.1 instead checks the archive commit touches no root agent file |
| A-R10 | WARNING (astra) | yes (`docs/development/spec-workflow.md:32-36`) | fix | 4.1 archive as last commit + reviewer re-check; 4.2 merge after 0.2/0.3, then per-checkout checksum check |
| O-1 | WARNING (opus) | yes | fix | PAC-1 gate list must equal `hack/check.sh` `run_gate` names; gate-list mutants (also makes change 6's own entry mechanical) |
| O-2 | WARNING (opus) | yes | fix | PAC-3 extraction rules: code-span paths, `<placeholder>` as `*`, globs must match, symbols declared in the named `.go` file; placeholder mutants |
| O-3 | WARNING (opus) | yes (`.gitignore` has no `worktrees/`) | fix | ignore scenario lists the exact seven paths; `worktrees/` stays a content-only check |
| O-4 | WARNING (opus) | yes | fix | hostname skip when it is a task name; whole-word match |
| O-6 | WARNING (opus) | yes | fix | `AGENT_CONTRACT_NESTED=1` stops the empty-pattern-set mutant recursing |
| O-7 | NOTE | yes | fix | PAC-5 lists which scenarios get mutants and the empty-pattern expectation |
| O-8 | NOTE | yes | fix | task names read from code spans/fences, flags and assignments skipped; parsing mutant |
| O-9 | NOTE | partly | fix in part | allowlist regexes evaluated against each path (incl. broad `.*\.md$`). Rejected: "removing `:10` un-exempts `AGENTS.local.md`" — that regex is anchored on `AGENTS\.md$` and never matched `AGENTS.local.md` |
| O-10 | NOTE | yes | fix | private-header check: first 10 lines, case-insensitive, named phrases |
| O-11 | NOTE | yes | fix | tasks say "at most 3 lines; one is enough" |
| O-12 | NOTE | believed (no network) | fix | guard fails closed on missing `rg`; 2.4 checks the runner and installs `rg` in `lint` if absent |
| O-13 | NOTE | yes | no action | `CLAUDE.local.md` ignore is covered by the ignore-rule mutant now; `CONTRIBUTING.md` and `setup.md` edits stay task-only (reviewed in 3.1) |

Accepted CRITICAL: A-R2, A-R4, A-R5 → one more round with fresh legs in `round-2/`.
