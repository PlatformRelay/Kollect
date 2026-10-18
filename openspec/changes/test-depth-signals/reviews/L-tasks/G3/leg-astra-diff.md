## Verdict
verdict: REQUEST_CHANGES
findings:
  - [P1] hack/test/readme_quickstart_test.sh:153 — `kubectl --context demo get foo` and `kubectl --context demo apply -k config/samples/nosuch/` pass: `demo` becomes the verb, bypassing validation. Counterargument: global flags are documented as unsupported; however, the claimed fail-closed behavior is false. Reject unsupported syntax or parse flag values.
  - [P1] hack/test/readme_quickstart_test.sh:129 — `kubectl get kinv & task nosuch` passes because single `&` is not split and processing stops after kubectl. Counterargument: current examples use `&&`; however, TDS-3 requires checking commands in the fence, including this valid shell command.
matrix_disputes:
  - None for the narrowly stated matrix rows; the separate “Known limits (fail closed)” claim is contradicted by the reproduced global-flag bypass.
could_not_check:
  - Read-only sandbox prevents scratch copies and mutation runs of `--self-test`; used in-memory README substitutions with only input plumbing adapted, confirming both bypasses and green/red controls.
  - Did not rerun the recorded broader sensors or deferred `task check`; repository-local AGENTS.md was absent, so supplied rules were used.
summary: Required cases and CI wiring are present; real README passes, but two extraction gaps silently accept invalid commands. No delegation-check duplication or evidence-format violation found.
