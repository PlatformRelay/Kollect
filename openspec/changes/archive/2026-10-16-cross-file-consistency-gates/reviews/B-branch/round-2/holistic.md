# Stage B round 2 — holistic fitness on the tip

Tip `800a33e69` (feat/cross-file-consistency-gates), base `0c1456b5d` (origin/main). Run 2026-10-16,
`umask 022`. mise's kubectl 1.37.0 (`~/.local/share/mise/installs/kubectl/1.37.0`) was prepended to
PATH for the whole run, because the worktree's mise shim has no kubectl version set (round-1 demo_04
red was exactly that). The two review legs ran in parallel on the same host.

| Command | Exit | Key line | Time |
|---|---|---|---|
| `task check` | 201 (task: exit status 1) | `task check: FAILED -- 1 gate(s) failed: lint:markdown`; the other 17 gates green, incl. format:check, lint, vulncheck, test, verify, gitleaks, workflow-security, every consistency_* guard plain and `--self-test`, demo_04 (`ok (2 tree(s) via kubectl kustomize)`) | 803 s |
| `go test -race -count=1 ./internal/...` (task check's `test` runs no race) | 0 | no FAIL line; covers `internal/metrics` incl. the two new registry tests | 94 s |

## The red gate

**lint:markdown — branch-caused, real, a regression after G7.** `Linting: 287 files` /
`Summary: 1 issue in 1 file`: `openspec/changes/cross-file-consistency-gates/loop.md:79 error
MD032/blanks-around-lists` — the `## Lessons` heading is followed directly by a new list item added in
`800a33e69` (the G7 record commit), after G7's last whole-repo lint run on `9203283bf`. Reproduced
with `docs/node_modules/.bin/markdownlint-cli2 --config hack/tooling/markdownlint-cli2.yaml --no-globs
.../loop.md`. `preflight` (required) runs `task lint:markdown`, so the PR would be red. H1 is therefore
not closed on the tip. (G7.md's final lint key line, `Summary: 0 issues in 0 files`, names no file
count, so it could not have proved coverage anyway.)

## Verification of the round-2 legs' CRITICAL / WARNING findings (orchestrator, against the code)

Legs: `leg-astra-adversarial.md` (BLOCK), `leg-claude-opus-spec.md` (CONCERNS). Probes ran the guard
on a `git archive HEAD` copy with one Dockerfile line changed.

| id | Sev (leg) | Finding | Verified? | Evidence |
|---|---|---|---|---|
| H1b | WARNING (astra) | loop.md:79 MD032, lint:markdown red | yes; CRITICAL (required gate red) | above |
| C1 | WARNING (opus) | `FROM golang:1.27.2@${D}` / `golang:1.27.2-${X}` pass CFC-1: the G7 `$` rule exempts `golang:`-prefixed images, the `@…` strip and `${tag%%-*}` remove the variable | yes; A1 not fully closed | probe: both → `ok - go.mod, every golang image ... agree`, exit 0. `consistency_go_toolchain_test.sh:95-96`, `:187`. Exemption added by G7 (`9203283bf`); no mutant |
| C2 | CRITICAL (astra) | `FROM \` + a comment line + `${B} AS builder` yields no image (Docker drops comment lines inside a continuation) | yes; severity WARNING (unusual hand edit); pre-existing parser hole, not from G7 | probe: exit 0; same with literal `golang:1.99.0` → exit 0. Parser `:79-86` joins the comment into the FROM line, takes `#` as the image |
| C3 | WARNING (opus) | registry-qualified golang (`mirror.gcr.io/library/golang:1.99.0`, `localhost:5000/golang:1.99.0`) prints no image line | yes; pre-existing, already deferred to BACKLOG by G7 | probe: both exit 0; only `docker.io/` / `library/` stripped at `:90` |
| C4 | WARNING (astra) | heredoc body `FROM ${B}` trips the `$` rule | yes; fails closed (false red, no bypass) | probe: `FAIL: Dockerfile: FROM ${B} names its image through a variable` |
| C5 | WARNING (astra) | `customResourceLabeledCollector{}` registered twice is accepted (unchecked collector) and both tests collapse it | yes; outside CFC-4's name invariant | client_golang `registry.go:343-344` appends unchecked collectors without a dup check; test uses a name set (`metrics_catalog_test.go:106-109`) and skips it at `:167` |
| C6 | WARNING (opus) | `tool-pins-selftest` not in protect-main | yes, unchanged from round-1 O1 (deferred post-merge row) | — |

NOTEs not verified: opus N1 (`$` rule also hits non-Go Dockerfiles; documented), N2 (G7.md:50-51
stale "green control" sentence contradicts :48), N3 (tasks.md Verification table still `not-run`).

## Round-1 fixes on the tip

| Finding | Closed? | Basis |
|---|---|---|
| H1 markdownlint | **no** | regressed in `800a33e69` (loop.md:79) |
| A1 variable FROM image | **partly** | the six named spellings fail (self-test mutants, both legs); `golang:<lit>@${D}` / `-${X}` still pass (C1) |
| A2 MustRegister drop | yes | both legs; Describe-based two-way comparison + AlreadyRegisteredError test; race green |
| A4 temp-dir leak | yes | both legs; `new_copy` in parent shell |
| O3 rbac traceback | yes | both legs; `partition(":")` + mutant; self-test green in task check |

Compliance (opus): CFC-1 partial (C1, C3), CFC-2 holds, CFC-3 holds, CFC-4 holds (stronger than
spec), CFC-5 holds, CFC-6 partial (C6).
