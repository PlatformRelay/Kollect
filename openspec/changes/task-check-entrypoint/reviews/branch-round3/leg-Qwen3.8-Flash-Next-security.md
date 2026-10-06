Verifying my awk-sub claim empirically (read-only, stdin only, no files touched):
## Verdict: CONCERNS

## Findings
- [WARNING] The CWS-2 key-extraction awk is dead code: `sub(/"?.*/, "", key)` always empties `key`, so the bare-`ignore:` exemption at `hack/test/ci_workflow_security_test.sh:284` is unreachable (`rest`/`value` at :282-283 are also dead — `sub` targets the never-assigned `value`). Verified with a standalone awk run: `key=[""]` for any input.
  Failure: the first real block-style suppression in `.github/zizmor.yml` (`# why` + rule + `ignore:` + commented items) reds with "suppression `ignore:` … has no comment" on a structural line; the format the comment block documents cannot pass. Green today only because `rules: {}`. The round-2 mutants pass because they fail in the stricter direction, masking the bug.
  Fix: replace with `key = $0; sub(/^[[:space:]]*"?([a-zA-Z0-9_-]+)"?:.*/, "\\1", key)` (capture form) and delete the `rest`/`value` lines; add a mutant asserting a justified bare-`ignore:` PASSES.
  Confidence: 90
- [WARNING] `run_gate go-mod bash -c 'go mod tidy && git diff --exit-code go.mod go.sum && go mod verify'` — `hack/check.sh:56`: `go mod tidy` mutates the tracked `go.mod`/`go.sum` (and the module cache) inside a gate advertised as a check. Failure: if `git diff` shows drift, or a later gate fails, or tidy aborts mid-fetch, the developer's tree is left modified/damaged by a read-only-looking command; `task_check_test.sh` pins this exact string so the mutation is contractual, and CI's lint job only runs the wiring guard, so nothing reviews tidy's effect.
  Fix: `cp` both files to a temp dir, tidy, `diff`, restore (or run tidy in a worktree); update the pinned command in `hack/test/task_check_test.sh:85` to match.
  Confidence: 85
- [NOTE] The Docker-guard skip is prose-matched (`grep -qi 'requires docker'` on the header, `hack/check.sh:82`): any committed guard can opt out of the sweep by one comment line, while `coverage: preflight=…,guard-sweep` (`hack/check.sh:66`) keeps claiming the sweep covers a required context. Currently only `integration_no_docker_test.sh` matches, and legitimately (its header declares it).
  Fix: require a structured token, e.g. a header line matching `^# *check-skip: docker`, matched exactly.
  Confidence: 75
- [NOTE] `hack/check.sh:91` decides "this guard parses `--self-test`" from textual presence of the string on any non-comment line. A guard that merely embeds the flag in a failure message would be run in a mode it does not parse. Today only `ci_workflow_security_test.sh` and `task_check_test.sh` match and both parse it.
  Fix: same structured-declaration approach as above when a false trigger first appears.
  Confidence: 70

Checked clean: local gitleaks invocation matches CI (`ci.yaml:168`, `--redact --no-git`, same config — secret values stay redacted in both); zizmor invocation matches `ci.yaml:194` byte-for-byte (offline, min-severity=high, same config); both installers SHA256-verify before executing, zizmor digests committed per-asset, `KOLLECT_FORCE_SHA256` override is test-only and local-effect only; CWS-3 narrows the dependency-review step allow-list (drops `shell:`) — a tightening, no allow-list widened anywhere in the range; mutants run only inside `mktemp -d` copies (`hack/test/ci_workflow_security_test.sh:714-731`), never the real tree; no new secrets handling or trust boundary; `task_check_test.sh` plain mode is grep/yq only, no eval of PR-controlled content.

## Could not check
- Did not read `openspec/changes/task-check-entrypoint/reviews/**` (prior review legs — out of bounds) nor `evidence/evidence.md`, so round-1/2 registers are unknown to this pass.
- Did not execute `task check` or the two new guard modes (plan mode, read-only); only the awk sub-semantics was run standalone.
- Did not verify each swept guard actually passes on a Dockerless machine (e.g. `e2e_webhook_existing_cluster_test.sh`, `finalizer_cleanup_e2e_test.sh` headers lack the Docker declaration) — a false-red risk, not traced to a security sink.
- `hack/lib/fetch.sh` / `verify-sha256.sh` internals not re-read (pre-existing, unchanged in range); gitleaks checksums.txt is same-origin TOFU (pre-existing).
