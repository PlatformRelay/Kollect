# R spec-set review: verification (round 1)

Change `mutation-testing-signal`, spec set at `9a5426f3a` (content of `fa3fdf389`), reviewed
2026-10-19. Target question: would executing `tasks.md` as written deliver the spec?

## Legs

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (spec lens, cold brief `brief-astra.md`) | `leg-astra-spec.md` | 0 | BLOCK |
| claude/opus:spec (fanout.sh, `--no-unify`) | `leg-claude-opus-spec.md` | 0 (171 s) | CONCERNS |

Both legs ran. No stand-in. The orchestrator verified every finding against the tree and the
gremlins v0.6.0 source in the module cache, and ran a scratch probe (2026-10-19, outside the repo:
a one-function module, gremlins v0.6.0 built by `go install` under go1.27.2). Every claim is tagged
verified (read or run, 2026-10-19) or believed.

Probe results (verified, run): plain run exit 0 (1 LIVED, 2 NOT COVERED); `--threshold-efficacy
100` exit 0; `GREMLINS_UNLEASH_THRESHOLD_EFFICACY=100` exit 0; a YAML threshold in `.gremlins.yaml`,
in `.gremlins.json` and in a bare `.gremlins` exit 10 each; the same `.gremlins.yaml` with
`--config=<file holding {}>` exit 0; `GREMLINS_MUTANTS_ARITHMETIC_BASE_ENABLED=true` and `=false`
both removed the ARITHMETIC_BASE mutants; `--dry-run` wrote RUNNABLE; `file_name` is relative to
the package directory (`p.go`); the installed binary reports version `dev`.

## Findings

| Id | Sev (after verification) | Legs | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| V1 | CRITICAL | astra (A1), opus | yes, probe | config ban covered only `.gremlins.yaml`/`.yml`; viper tries every extension and the bare name, decoded as YAML | fix |
| V2 | CRITICAL | astra (A2) | yes, probe | 1.1 expected `--threshold-efficacy 100` to exit non-zero; in v0.6.0 CLI and env thresholds are inert (string value, typed lookup reads 0); opus's "checked and holding" claim of the opposite is wrong | fix |
| V3 | CRITICAL | astra (A3), opus | yes | fail-open scans: `env:` from a `${{ }}` expression or a Taskfile `dotenv:` never names the variable; flag spellings `_`/`.` normalised to `-` (`cmd/unleash.go:196-205`); scan errors not required to fail | fix |
| V4 | CRITICAL | astra (A4) | yes | W1/W2 named commands but not exact ones: `task mutation \|\| true` or an extra step passes every case | fix |
| V5 | CRITICAL | astra (A5), opus (NOTE) | yes, probe | env can narrow the run without touching argv: any `GREMLINS_MUTANTS_<T>_ENABLED` value switches a mutator off; `GREMLINS_UNLEASH_DIFF`/`_TAGS`/`_COVERPKG` are read as strings | fix |
| V6 | WARNING | astra (A6) | yes | no fresh-report rule, no stale-report case; invalid report not required to block a partial table; gremlins exits 0 with no report on nothing-to-report or cancel (`report.go:255-260`, `unleash.go:135-136`) | fix |
| V7 | WARNING | astra (A7) | partly | M7 needed the runner (moved to R7); redundant checks let mutants survive by exit code (cases now judged by the runner's own message); no fixture held TIMED OUT (synthetic all-status report added). Rejected part: W1 against a missing file is a valid red through the guard's absence message (precedent: change 7's absence mutant), now stated in the tasks preamble | fix (part), reject (part) |
| V8 | WARNING | astra (A8) | yes | "no new proxy dependency on a required check" is false: CFC-2 in `lint` (`ci.yaml:619-621`) resolves the new pin with `go mod download` | fix: proposal wording, cold/warm CFC-2 timing in 2.5 |
| V9 | WARNING | astra (A9), opus | yes | W2's `gremlins@` scan covered only `.github/`, `hack/ci/`, `hack/task/`; a Makefile or `hack/*.sh` install passes; CFC-2 does not compare pins (`consistency_tool_pins_test.sh:155-188`); pin shape must be a two-space top-level `vars:` entry (`:129-132`) | fix (W3; W2 shape) |
| V10 | WARNING | astra (A10) | yes | "no other file SHALL name a gremlins version" contradicts fixture provenance | fix |
| V11 | WARNING | astra (A11) | yes | formatter interface and control identity unfixed; no duplicate/missing package or absent-control-while-another-LIVED case | fix (MUT-1 interface, MUT-3 identity, M8, R4) |
| V12 | WARNING | astra (A12) | yes | 2.3 bundled well over ~15 claims | fix (split into 2.3 and 2.4) |
| V13 | NOTE | opus | yes | example env name `GREMLINS_UNLEASH_THRESHOLD_MCOVER` is not read; the key is `unleash.threshold.mutant-coverage` (`configuration.go:49`) | fix |
| V14 | NOTE | opus | yes | M4 cannot happen in v0.6.0 (no results, no report) | fix: M4 kept as a format guard, said so; R3 covers today's tool |
| V15 | NOTE | opus | yes | "lint SHALL NOT install gremlins" untested | fix (W3 mutant) |
| V16 | NOTE | astra (A13) | yes | reporter edits and cited lines consistent with the tree | no action |

## Fixes applied

- The runner passes `--config=hack/ci/gremlins.yaml` (committed, `{}`) and all eleven mutator
  flags, refuses any `GREMLINS_` variable at run time, and writes to a fresh `mktemp -d`
  directory (proposal *What Changes*, MUT-1, MUT-2; R1, R3, R6).
- Config ban: `.gremlins` and every `.gremlins.*`, and the committed config must stay `{}` (MUT-2,
  T2).
- Threshold flag scan matches `--threshold[-_.]`; scans fail closed (MUT-2, T1, C3).
- W1 pins the exact step list and the exact `run:`/`env:` of the run step; W2 pins the exact
  Taskfile commands, deps and the pin shape; W3 scans CFC-2's file set for any install or run
  outside the pin.
- 1.1 probe expectations corrected (inert CLI/env thresholds, exit 10 for YAML, `--config`
  override, env disabling mutators, `dev` version string); proposal *Assumptions* updated.
- Formatter validates before printing; bad report last in each case; M8 package-argument cases;
  R4 control identity (file, line, column, type) exactly once.
- 2.3 split into 2.3 (workflow and install chain) and 2.4 (threshold sources and reporter);
  implementation is 2.5, defect controls 2.6; `task-prompt.md` dispatch group updated.

CRITICAL accepted (V1-V5), so one second round runs with fresh legs in `round-2/`.
