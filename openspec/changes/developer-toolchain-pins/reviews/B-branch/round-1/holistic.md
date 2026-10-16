# Stage B round 1: holistic run

Branch `feat/developer-toolchain-pins` at `8383c5995`, base `966e17e10`. `origin/main` moved during B
to `dfe032184` (one commit, `docs(spec): archive cross-file-consistency-gates`); `git merge-tree`
against it is clean.

Setup: `make operator-sdk` (fresh worktree, v1.42.3 into `bin/`), mise kubectl 1.37.0 on `PATH`,
`umask 022`, both runs backgrounded and waited on by PID. `/tmp` at 71% inodes and 30% space before
the run; no ENOSPC.

| Command | Exit | Duration | Notes |
|---|---|---|---|
| `task check` | 0 | 1528s | verify, build, lint, lint:shell, lint:markdown, arch-lint, format:check, test:run (47 packages), helm-lint, helm-unittest, helm-docs:verify, audit:rbac, scrub, spec:validate, vulncheck; 1077 `ok` lines; 18 toolchain-pins lines; dist_* and demo guards ran; 0 `FAIL` |
| `task vulncheck` | 0 | 12s | "No vulnerabilities found" (0 affecting code) |

## Bump simulation (to verify the astra finding)

Scratch copy of the tracked tree (since removed), git-cliff v2.13.1 to v2.14.2 at its three sites
(`Taskfile.yml:22`, `hack/verify-changelog.sh:11`, `.github/workflows/release.yaml:420`):

| Command | Exit | Result |
|---|---|---|
| `consistency_toolchain_pins_test.sh` (plain) | 0 | sites agree |
| `consistency_toolchain_pins_test.sh --self-test` | 1 | `git-cliff with: before uses:` red for the wrong reason: sentinel expects `v2\.13\.1`, the gate printed `v2.14.2` |
| same, sentinel at `:1285` patched in the copy, plus Task 3.52.0 to 3.53.0 at 19 sites | 0 | "All toolchain-pin consistency self-tests passed" |
| `ci_workflow_security_test.sh`, `ci_fetch_lib_hardening_test.sh` | 0 | green |
| `consistency_go_toolchain_test.sh` | 0 | green, once mise's real jq was on `PATH` (the jq shim fails in a scratch directory) |
| `consistency_tool_pins_test.sh` | 1 | `go mod download` could not resolve gum/logtools in the scratch copy; it is an artifact of the copy (the guard does not read Task or git-cliff, and it's green in `task check`) |
