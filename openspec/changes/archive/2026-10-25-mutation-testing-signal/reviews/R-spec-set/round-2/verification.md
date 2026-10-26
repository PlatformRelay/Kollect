# R spec-set review: verification (round 2, last)

Change `mutation-testing-signal`, spec set at `2b14ee6c2` (round-1 fixes applied), reviewed
2026-10-19. Target question: would executing `tasks.md` as written deliver the spec? This is the
last round: its verified mechanical findings are fixed below with no further review.

## Legs

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (spec lens, cold brief `brief-astra.md`) | `leg-astra-spec.md` | 0 | BLOCK |
| claude/opus:spec (fanout.sh, `--no-unify`) | `leg-claude-opus-spec.md` | 0 (276 s) | CONCERNS |

Both legs ran, fresh. No stand-in. Round-1 closure per astra: V1, V2, V6, V7, V8, V10, V11, V13,
V14 closed; V3, V4, V5, V9, V15 partly closed; V12 not closed. Opus: V1-V6 hold against the
gremlins source. The open parts are the findings below. Every claim is tagged verified (read,
2026-10-19) or believed.

## Findings

| Id | Sev (after verification) | Legs | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| R2-1 | CRITICAL | astra (A1) | yes: `Taskfile.yml:80-85` uses `sources:`/`generates:`; Task skips on them independently of `status:` | W2 left `sources:`, `generates:`, `method:` and command-level `ignore_error:` open, so `task mutation` could be skipped green | fix: MUT-1 and W2 forbid them; mutants |
| R2-2 | CRITICAL | astra (A2), opus | yes: a `lint` step `run: task mutation` names neither `go-gremlins` nor a `gremlins` command word; CFC-2 does not follow `includes:` (`consistency_tool_pins_test.sh:175-188`) | W3 could not see gremlins reached from a required job through Task, and its scan set was defined by reference | fix: W3 defines its own scan set (includes followed recursively, all of `.github/`), bans references to the install task, `task mutation`, a `mutation` dep and the runner outside their sites; mutants |
| R2-3 | CRITICAL | astra (A3), opus | yes: W1 pinned no upload `path:`; the runner's `mktemp -d` path was unknown to the workflow; default `if-no-files-found` is `warn` (believed, action default) | MUT-1's "report artifact exists" could not hold | fix: runner writes `reports=<dir>` to `$GITHUB_OUTPUT` and keeps the dir; W1 pins `id: mutation`, `path: ${{ steps.mutation.outputs.reports }}`, `if-no-files-found: error`; R8 tests the hand-off |
| R2-4 | WARNING | astra (A4) | yes | C3 covered two scan errors; YAML alternate forms unnamed | fix: C3 covers every YAML file read and an unreadable scan-set file; guard reads YAML with anchors expanded; T3 adds flow map, quoted key, alias |
| R2-5 | WARNING | astra (A5) | yes | R6 sampled three names, so a fixed denylist passes | fix: R6 adds `GREMLINS_UNLEASH_TAGS`, an unrelated `GREMLINS_KOLLECT_PROBE`, an empty exported variable; 2.8 mutants for a fixed list and for ignoring empty values |
| R2-6 | WARNING | astra (A6) | yes | 2.2 and 2.3 still too large; W1/W2 mutants unnamed | fix: runner cases split (2.2 measurement, 2.3 env/nights/hand-off); workflow (2.4), install chain and scope (2.5), thresholds and scan errors (2.6); W1 and W2 mutants enumerated; implement is 2.7, controls 2.8 |
| R2-7 | WARNING | opus | yes for the relative `BIN_DIR` (`Taskfile.yml:18`); believed for `go install` refusing a relative `GOBIN` | nothing ran the real chain before merge | fix: W2 uses `{{.ROOT_DIR}}`; R1 pins `<short-pkg>.json`; 2.7 runs `task mutation` locally plain and forced |
| R2-8 | WARNING | opus | yes: `ci_workflow_security_test.sh:56-59` exits on first failure | a T mutant that also breaks W1/W2 would red with the wrong message | fix: tasks preamble requires the guard to collect every failure (MUT-2 says so too) |
| R2-9 | NOTE | opus | yes: every `cfc_guards` entry has a CWS-7 unwired mutant (`ci_workflow_security_test.sh:1104-1144`) | the new entry had none | fix: 2.7 adds one like `:1136` |
| R2-10 | NOTE | opus | yes: the fixture call runs from its own directory | relative `--config` would not resolve there | fix: absolute path in MUT-1 and R1; 2.8 mutant |
| R2-11 | NOTE | opus | yes: `cmd/unleash.go:65` `threshold-mcover` | T1 used a non-existent spelling only | fix: `--threshold-mcover` and `--threshold.mcover` |
| R2-12 | NOTE | opus | yes | 1.1 recorded the control under an unnamed argv | fix: recorded under the runner's exact argv |
| R2-13 | NOTE | astra (A8) | yes: `configuration.go:147-148` | the current directory is also a config search path | fix: listed in proposal *Risks*; `--config` closes it |

No finding was rejected. No CRITICAL remains open: R2-1 to R2-3 are fixed in the spec set as
described, unreviewed (last round). Reviewer attention at stage B: the W2/W3 bans and the
`$GITHUB_OUTPUT` hand-off are new in this round.
