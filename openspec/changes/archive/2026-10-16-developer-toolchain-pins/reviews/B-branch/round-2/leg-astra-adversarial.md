## Verdict: BLOCK

## Round-1 closure

- 1: PARTIAL — `hack/test/consistency_toolchain_pins_test.sh:1262` — Current-value sentinels are dynamic; bump counts, extracted sites and nested exit are checked, but an inherited environment variable bypasses the entire bump control.
- 2: CLOSED — `.github/workflows/ci.yaml:599`; `hack/test/ci_workflow_security_test.sh:621` — Self-test runs in `tool-pins-selftest`, with a 45-minute timeout; placement and `--self-test` are explicitly required.
- 3: CLOSED — `renovate.json:187` — The dedicated depName rule follows the shared group and has no later matching override; this follows [Renovate’s rule precedence](https://docs.renovatebot.com/configuration-options/#packagerules).
- 4: CLOSED — `hack/install-git-cliff.sh:45`; `hack/lib/verify-sha256.sh:22` — Correct algorithms are selected; absent tools and empty computed digests cannot satisfy the installers’ comparisons.
- 5: CLOSED — `hack/install-helm-docs.sh:49`; `hack/install-git-cliff.sh:54` — Full lowercase digests required; uppercase and duplicate matching lines fail closed. git-cliff accepts `*` markers; helm-docs rejects them.
- 6: PARTIAL — `hack/test/consistency_tool_pins_test.sh:126` — Taskfile variables require v-prefixed three-part versions, permitting prerelease/build suffixes; literal `latest` and Makefile-variable pins bypass that validation.
- 7: PARTIAL — `hack/test/consistency_toolchain_pins_test.sh:225` — Task/git-cliff matching and mise `latest` coverage are fixed; another action reader still matches case-sensitively.

## Findings

- [CRITICAL] CFC-2 still excludes literal `@latest` from inspection and permits floating Makefile variables — `hack/test/consistency_tool_pins_test.sh:162`
  Failure: Add `go run example.com/tool@latest` alongside valid pins: the scanner discards it. A `go-install-tool` version variable holding `v1` also bypasses `PIN_VERSION_RE` at line 147 and reaches module resolution as a floating query.
  Fix: Remove the `latest` skip and apply full-version validation to every scanner branch before resolution.
  Confidence: 99

- [WARNING] An inherited variable disables the required all-pins bump proof — `hack/test/consistency_toolchain_pins_test.sh:1262`
  Failure: `KOLLECT_TP_NESTED=1 bash hack/test/consistency_toolchain_pins_test.sh --self-test` skips both the bumped-tree check and its nested self-test, yet can finish successfully.
  Fix: Reject inherited bypass state at the public entry point; invoke the nested case runner through a separate internal path.
  Confidence: 100

- [WARNING] Mixed-case setup-go steps remain invisible to the Go consistency guard — `hack/test/consistency_go_toolchain_test.sh:148`
  Failure: Keep one normal setup-go step and add `uses: Actions/Setup-Go@…` with `go-version-file: .go-version`; both the reader and counter ignore the added step, leaving its divergent Go source unchecked.
  Fix: Match action references case-insensitively in both readers and add a mixed-case mutant.
  Confidence: 98

## Could not check

- The sandbox denied `/tmp` writes. Toolchain plain mode and checksum tests stopped at `mktemp`; workflow-security passed its initial checks but stopped before completing CWS-6.
- Full self-tests, scratch mutants, installer execution and per-commit green status were not verified. Commit order was inspected, but no intermediate tree was executed.
- No live Renovate extraction/update run or stock-macOS execution was performed; grouping and checksum conclusions are from code inspection.
