# Stage B round 2: verification of leg findings

Verified by the B orchestrator against HEAD `481bde067` (G6 range `b1f5a4369..HEAD`). Line
numbers are at that commit. Legs: astra adversarial (`leg-astra-adversarial.md`, BLOCK) and
claude/opus:spec (`leg-claude-opus-spec.md`, CONCERNS).

| Id | Sev (leg → verified) | Leg | Verified? | Finding | Proposed disposition |
|---|---|---|---|---|---|
| A1 | CRITICAL → WARNING | astra | partly | `hack/test/consistency_tool_pins_test.sh:145-146`: a Makefile `go-install-tool` version variable is resolved by `make_var` and printed as-is, never matched against `PIN_VERSION_RE` (`:127`), so `X_VERSION ?= v1` still floats; ed9d6cd76 only claims literal and Taskfile-var pins. The other half (literal `@latest` skipped at `:158`) is pre-existing on base `966e17e10` (`:158` there too) and deliberate: help text such as `hack/migrate-chart-path.sh:342` echoes `go install ...@latest` | fix (one line: apply `PIN_VERSION_RE` in the Makefile branch, plus a mutant) or record as a known CFC-2 gap; the `@latest` skip: reject, out of scope (change 4 design) |
| A2 | WARNING → NOTE | astra | yes | `hack/test/consistency_toolchain_pins_test.sh:1262`: an inherited `KOLLECT_TP_NESTED` skips the all-pins bump control. Nothing in `.github/` or the Taskfile sets it (only `:1280`), so CI is not affected | accept; optional hardening: pass the nested flag as an argument instead of the environment |
| A3 | WARNING → NOTE | astra | yes | `hack/test/consistency_go_toolchain_test.sh:148,158` match `actions/setup-go@` case-sensitively; a mixed-case second step escapes the Go guard. Pre-existing guard of another change; round-1 item 7 named only the Task and git-cliff readers, both fixed (`consistency_toolchain_pins_test.sh:225`, callers `:1042`, `:1062`) | follow-up outside this change |
| O1 | WARNING | opus | yes (extraction), update proposal not run | Renovate 44.115.4's built-in github-actions manager DOES extract `node-version`/`python-version` of `actions/setup-node`/`setup-python` (depType `uses-with`, `renovate/dist/modules/manager/github-actions/known-actions/github-releases.js:16-30`, `dep-types.js:28-29`). Sites: `.github/workflows/preflight.yaml:37`, `docs.yaml:106,118`. Such a bump would land in the `github actions` group (`renovate.json` packageRules[2]) without `mise.toml`, so the mise guard reds that group PR; the model and the bump control do not see these deps (G3 recorded non-custom managers as not modelled), and the `ci.yaml:563` comment "nothing else manages the node/python inputs" is wrong | fix cheap: a packageRule that disables (or groups alone) `matchDepTypes: ["uses-with"]`, and correct the comment; or record as a known gap with a follow-up |
| O2 | WARNING → NOTE | opus | yes | `consistency_toolchain_pins_test.sh:634-637` states the 44.115.4 dry-runs were not re-run after golangci-lint got its own group; `extends` presets are not modelled. `renovate.json:187-191` is the last rule matching that depName, so by rule order it wins | accept; optionally re-run the `lookup` dry-run before merge |
| O3 | NOTE | opus | yes | `PIN_VERSION_RE` also rejects literal partial pins (`@v1.9`), a semantic change to change 4's guard | accept; it is stated in the ed9d6cd76 body |
| O4 | NOTE | opus | yes | `tool-pins-selftest` now runs two serial self-tests (15-25 min per the `ci.yaml:594-598` comment) under `timeout-minutes: 45` | accept; watch first CI timings |
| O5 | NOTE | opus | yes | `hack/install-helm.sh:71`, `hack/install-polaris.sh:47` call `sha256sum` directly (fail closed on macOS, outside DTP-6) | follow-up |
| O6 | NOTE | opus | resolved | intermediate commits unverified per `evidence/G6.md` | closed by holistic.md: every commit of the range green on the cheap gate |

## Round-1 fix closure

| Round-1 item | Status | Evidence |
|---|---|---|
| B1 P1 sentinel + bump control | closed | `GC_CUR`/`GV_CUR` read from the tree; bump control `:1209-1285`, nested exit checked at `:1280-1282`, empty extraction fails at `:1267`; env bypass A2 is a NOTE |
| B2 self-test placement | closed | `ci.yaml:615-616`; CFC-6 loop `ci_workflow_security_test.sh:620-624` with a move-back mutant per script |
| golangci-lint group | closed | `renovate.json:187-191`; `ALONE` check `:899-956` (real-Renovate re-run not done, O2) |
| macOS fallback | closed | `install-git-cliff.sh:45-52`, `verify-sha256.sh:22-29`, self-test cases with hidden PATH |
| 64-hex / 128-hex digest | closed | `install-helm-docs.sh:49`, `install-git-cliff.sh:54` |
| CFC-2 full semver | partial | Taskfile-var and literal pins closed; Makefile-variable branch unvalidated (A1) |
| case-insensitive `uses:` + `task = "latest"` | closed for this change's readers | `:225`; A3 is another guard |
