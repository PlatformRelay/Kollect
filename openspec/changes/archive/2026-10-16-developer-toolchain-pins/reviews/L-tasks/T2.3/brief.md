# Review brief — task 2.3 of developer-toolchain-pins (diff-only, independent)

You are reviewing one task, independently of whoever implemented it. Read-only: do not edit files.
Repo root: the current directory (kollect, branch `feat/developer-toolchain-pins`).

1. TASK — `openspec/changes/developer-toolchain-pins/tasks.md` item 2.3 (full text also in the
   evidence file under `## Task text`): add the DTP-5 comment rule to
   `hack/test/consistency_toolchain_pins_test.sh` — a version-bearing comment in `mise.toml` or
   `Taskfile.yml` is allowed only in the contiguous comment block directly above its pin, or inline
   on the pin's line, naming that pin's tool (key or CI input name) at the pin's value (`v`
   stripped); any other version restatement and any pin-site count fails. Self-test cases:
   `helm (v3.21.4)`, `go 1.27.2`, "all 17 occurrences" → red; stale `version: 3.51.1` above `task`
   → red; Node block detached by a blank line → red; `gitleaks 8.30.1` above `node` → red; real
   Node and Python blocks → green; Taskfile `helm v3.22.0` above `HELM_VERSION` → green, `helm
   v3.21.4` elsewhere → red. Record plain mode red on `mise.toml:24, :28, :39, :52`, then rewrite
   those comments as pointers. One commit.
   Spec: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md` (DTP-5 at
   ~:120-162, DTP-7). Repo rules: AGENTS.md, docs/development/spec-workflow.md.
2. MATRIX and SENSORS — `openspec/changes/developer-toolchain-pins/evidence/T2.3.md` (design
   reading, matrix, red before the rewrite, the rewrite, sensors, hand mutants, test changes,
   deviations, harness gaps).
3. DIFF — the working tree against base 64fe97025: `git diff 64fe97025` (nothing committed yet; the
   evidence file and this brief are intent-to-add). Files: `mise.toml`,
   `hack/test/consistency_toolchain_pins_test.sh` (new `comment_findings` / `check_comments` after
   `check_mise`, new self-test block `# --- DTP-5 comments ---`, four ported DEV-MISE-01 mutants
   adjusted), `.github/workflows/ci.yaml` (one lint step name and its comment).
4. RULES — guards: plain mode checks the real tree, `--self-test` corrupts throwaway copies per
   mutant and asserts non-zero WITH the clause's own message and no other clause tripped, plus a
   no-op copy that must pass. Tests written by an earlier task (the ported DEV-MISE-01 mutants)
   change only with evidence from the spec, listed under `## Test changes`. The awk must stay POSIX
   (CI runners may use mawk or gawk).

Judge, in order:
a. INTENT — all of 2.3 and nothing else? Does `comment_findings` implement the spec's attachment
   rule (contiguous block directly above, or inline; names the pin's tool; equal value; any count
   fails)? Are there comments it misreads in today's `mise.toml` / `Taskfile.yml` (false red), or
   restatements it misses that the spec clearly covers (false green)? Do the rewritten `mise.toml`
   comments still say true things (check them against `Taskfile.yml`, `hack/install-helm.sh`,
   `.github/`)?
b. MATRIX — is each sensor real, would it fail if the claim were false? Mutation-verify in a scratch
   copy (mktemp, never the repo) any clause you doubt: `bash <guard copy> --self-test <repo>` runs
   in about 25 s. Does each probe prove its mutant landed? Do the four adjusted ported mutants still
   test what their labels say?
c. RULES — CI wiring (the lint step was renamed; CWS-6 / CFC-6 must still hold the guard in both
   modes); workflow security (only one step name and comment changed in YAML).
d. AI-code failure modes — tautological probes, weakened assertions, dropped halves, comments or
   evidence that claim more than the code does.

For each P0/P1, argue the opposite for one line; keep only what survives. An APPROVE with no
findings is valid when substantiated.

RETURN at most 15 lines, nothing else, under a `## Verdict` heading:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> — <why>
could_not_check:
  - <what>
summary: <one line>
