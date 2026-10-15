You are reviewing a set of fix commits on a git branch, independently of whoever wrote them, with an
ADVERSARIAL lens. You run read-only in the repository root. You have not seen the authors'
reasoning; do not look for it in `openspec/changes/cross-file-consistency-gates/loop.md`,
`evidence/` or `reviews/` (those are the authors' narrative). Judge the code.

## Inputs

1. CONTRACT — OpenSpec change `cross-file-consistency-gates`, requirements CFC-1..CFC-6 in
   `openspec/changes/cross-file-consistency-gates/specs/cross-file-consistency/spec.md` (read it).
   CFC-1: go.mod's `go` directive equals every `FROM golang:` image in `Dockerfile*` and the
   devcontainer image. CFC-3: chart-rendered manager roles cover `config/rbac/role.yaml`, with a
   reasoned exceptions file. CFC-4: metric names agree between `internal/metrics`, the docs, the
   chart PrometheusRule and the `registeredMetricNames` mirror. CFC-6: every guard has a self-test
   whose mutants must go red for the named reason.
2. SCOPE — the fix range `git diff 028bbc44c..HEAD` (ignore `openspec/` except the spec). Files:
   `hack/test/consistency_go_toolchain_test.sh`, `hack/test/consistency_rbac_test.sh`,
   `internal/metrics/metrics.go`, `internal/metrics/metrics_catalog_test.go`,
   `internal/metrics/metrics_test.go`, plus whitespace-only markdown fixes under
   `openspec/changes/cross-file-consistency-gates/` (check only that they changed no meaning).
3. THE PREVIOUS ROUND'S FINDINGS these commits claim to close. Decide for EACH whether it is
   really closed, on the code, not on the commit message:
   - H1: `task lint:markdown` (markdownlint-cli2, config `hack/tooling/markdownlint-cli2.yaml`)
     was red with MD032/MD038 in this change's `loop.md`, `task-prompt.md`, `evidence/G1..G6.md`.
     Is every tracked markdown file the config covers now clean, and does the config still cover
     them (no new ignore entry, no glob that now matches zero files)?
   - A1: `ARG B=golang:1.99.0` + `FROM ${B}` passed the CFC-1 guard. Claimed fix: any `$` in a
     FROM image fails, except a `golang:`-prefixed image whose variable tag fails the literal-
     version check. Attack it: `FROM --platform=$BUILDPLATFORM golang:1.99.0`, `FROM ${B} AS x`,
     lowercase/uppercase `from`, line continuations (`FROM \` newline image), `FROM` in a heredoc or
     comment, `docker.io/library/golang:1.99.0`, `golang@sha256:…`, `localhost:5000/golang:x`,
     `ONBUILD FROM`, a Dockerfile under a name the glob misses (`*.Dockerfile`, `Containerfile`),
     `# syntax=` lines, `COPY --from=golang:1.99.0`.
   - A2: the metrics guard counted any `kollect_*` literal as registered, so dropping a collector
     from `MustRegister` stayed green. Claimed fix: `Register()` delegates to
     `registerTo(prometheus.Registerer)`; a Go test feeds a recording Registerer and compares
     names (from each collector's `Describe`) with `registeredMetricNames`; a second test proves
     `Register()` fills controller-runtime's `metrics.Registry` via `AlreadyRegisteredError`.
     Attack it: can a collector be dropped, renamed or registered twice while both tests stay
     green? Is the name extraction from `Desc.String()` robust or would it silently skip? Is the
     comparison two-directional? Are the unchecked/labeled collector special cases a hole? Do
     the tests share global state (sync.Once) in a way that hides a failure or breaks `-count`,
     `-shuffle`, `-race` or parallel runs?
   - A4: the go_toolchain `--self-test` leaked one temp dir per mutant (copy made in a `$(...)`
     subshell, so the EXIT trap never saw it). Claimed fix: `new_copy` sets a global and is
     called in the parent shell. Is every copy now removed, on success AND failure paths?
   - O3: the rbac guard died with a Python traceback on an exception line whose class token had
     no colon (e.g. `cluster-scoped` instead of `cluster-scoped: reason`). Claimed fix:
     `partition(":")`. Is it now a clear FAIL line, and is an empty reason still rejected?
4. RULES — `AGENTS.md` and `agent-context/GUIDELINES.md` if present: fail closed, no silent
   fallbacks, tests must be able to fail.

## What to attack

a. For each fix: a realistic edit that breaks the invariant while the gate stays green.
b. Each new self-test mutant: does it go red for the reason it names, or could the clause be
   deleted and the self-test still pass? A `sed` that does not match leaving the copy unchanged?
c. Regressions: did a fix make a guard reject something valid in the tree today, or break another
   self-test case, or change runtime behaviour of `Register()`?
d. AI-code failure modes: tautological tests, weakened assertions, prose claiming more than code.

Mutation-verify in your head or with read-only probes; you cannot write files. For each
CRITICAL/WARNING, argue the opposite for one line; keep only what survives. An APPROVE with no
findings is valid when substantiated.

## Return (at most 25 lines, nothing else)

verdict: APPROVE | REQUEST_CHANGES | BLOCK
closed:
  - H1: yes|no — <one line>
  - A1: yes|no — <one line>
  - A2: yes|no — <one line>
  - A4: yes|no — <one line>
  - O3: yes|no — <one line>
findings:
  - [CRITICAL|WARNING|NIT] <one line, with path:line> — <how the invariant breaks while the gate stays green, or the concrete risk>
could_not_check:
  - <what you could not verify>
summary: <one line>

Severity: CRITICAL = a guard can pass with its invariant broken on a realistic change, or a vacuous
gate presented as proof. WARNING = correctness gap, untested behaviour, or a fail-open path needing an
unlikely trigger. NIT = note only.
