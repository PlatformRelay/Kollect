## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] hack/test/readme_quickstart_test.sh:128 — Backslash-continued `kubectl` commands bypass checks: `kubectl \<newline>get kinv,foo` and `kubectl apply \<newline>-k config/samples/nosuch/` both pass. Counterargument: current examples occupy single lines; TDS-3 nevertheless requires checking fence commands. Join continuations or reject them.
  - [P1] hack/test/readme_quickstart_test.sh:105 — A two-space-indented `sh` fence containing `kubectl get foo`, added alongside existing fences, passes unchecked. Counterargument: current fences start at column zero; indented fences remain valid Markdown within TDS-3’s scope.
matrix_disputes:
  - Narrow verification-matrix rows have meaningful sensors; however, G3.md:121 claims 27 mutation kills while its reproducible script contains only 25 mutants.
could_not_check:
  - Read-only filesystem prevents scratch copies and `--self-test` mutation reruns; reproduced findings and round-1 fixes using in-memory README substitutions with input plumbing adapted.
  - Broader recorded sensors, CI execution, and deferred `task check` were not rerun.
summary: Both round-1 fixes hold and the real README passes; required cases and CI wiring are present, but two extraction gaps remain. No delegation duplication or evidence-format violation found.
