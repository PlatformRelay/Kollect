# Review brief — task 2.2 of developer-toolchain-pins, round 2 (diff-only, independent)

You are reviewing fixes to one task, independently of whoever implemented them. Read-only: do not
edit files. Repo root: the current directory (kollect, branch `feat/developer-toolchain-pins`).

1. TASK — `openspec/changes/developer-toolchain-pins/tasks.md` item 2.2 (full text in the evidence
   file under `## Task text`). DTP-2 in
   `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md`: `Taskfile.yml`
   defines `GOVULNCHECK_VERSION` under `# renovate: datasource=go depName=golang.org/x/vuln` and
   `task vulncheck` uses it.
2. ROUND 1 found, and the fix claims to close (details in the evidence under `## Independent review`):
   - `GOVULNCHECK_VERSION: latest` passed the guard → the value must now be exactly
     `  GOVULNCHECK_VERSION: vN.N.N`;
   - a task-level `vars: GOVULNCHECK_VERSION: …` under `vulncheck:` overrode the annotated pin
     unseen → any `GOVULNCHECK_VERSION:` key inside the vulncheck block now reds;
   - the definition lookup matched the same key under `env:` → now scoped to top-level `vars:`;
   - trailing blanks after the annotation passed, though `renovate.json:32` would not extract → exact.
   Each has a new `--self-test` case and a hand mutant recorded in the evidence.
3. DIFF — `git diff 9fab7adae -- hack/test/consistency_toolchain_pins_test.sh`, focused on
   `check_govulncheck` and the `# --- DTP-2 govulncheck ---` self-test cases. The rest of the
   task's diff (Taskfile.yml, ci.yaml, consistency_tool_pins_test.sh) was approved in round 1 and is
   unchanged since.
4. RULES — `--self-test` corrupts throwaway copies per mutant and asserts non-zero WITH the clause's
   own message and no other clause tripped; a no-op copy must pass. Recorded run: exit 0, 61 `ok`.

Judge: does each fix close its finding without opening a new bypass or a false red on the real
tree; does each new case's probe prove its mutant landed and its sentinel name only its own clause;
does any comment or evidence line claim more than the code does? You may mutation-verify in a
scratch copy (mktemp, never the repo): `bash <guard copy> --self-test <repo>` takes about 60 s.

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
