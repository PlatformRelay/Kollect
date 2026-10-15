## Verdict: CONCERNS

All six requirements are implemented and each has a no-op control plus mutants. Only CFC-6 has a real gap: the new `tool-pins-selftest` job is not yet a required status check on GitHub. The rest of the spec holds or is enforced more strictly than written.

**Compliance table**
- **CFC-1: holds.** The image versions are compared with go.mod at `hack/test/consistency_go_toolchain_test.sh:173-189`. A `toolchain` line is rejected at :162. CI literals and setup-go steps are checked at :206-223. An empty scan fails at :169. The Renovate group, its order and the `rangeStrategy: bump` rule are checked at :232-280, and the matching rules are in `renovate.json:108-115,141-145`. Stronger than spec: images must agree with each other (:197), and the devcontainer must use a golang image (:191).
- **CFC-2: holds.** `GOTOOLCHAIN=go<go.mod>` is exported at `consistency_tool_pins_test.sh:332` and checked at :334. The tool's go directive must not exceed go.mod's (:367-373). Lookups fail closed (:350-364). Each analysis tool gets a smoke run (:385-401), and a missing pin fails (:403-411). Weaker than the wording: golangci-lint and govulncheck only run against `SMOKE_PKG` (one package, :76), not "the module". Stronger than spec: the package must exist in the module (:222-227), and `.custom-gcl.yml` must name the same golangci-lint version as the Makefile (:377-382).
- **CFC-3: holds.** Coverage is checked at `consistency_rbac_test.sh:236-243` and missing reasons at :176. The tenant classes are checked against CRD scope and a fixed list of workload kinds (:185-203, :85). Stronger than spec: every exception needs a mode column, and an exception the render no longer needs fails (:244-247). In default mode only a ClusterRole counts, even though the spec's "manager roles" includes the leader-election Role (:80). That is correct RBAC behaviour.
- **CFC-4: holds.** The registered set comes from scanning the source, with `metrics_catalog.go` left out (:131-145). Docs and rule names must be registered, with only the three histogram suffixes allowed (:181-201). Every registered metric must be in metrics.md (:202) and the mirror list must match (:206-221). Stronger than spec: a `Namespace:` or `Subsystem:` field fails (:148), and so does any Prometheus client import outside `internal/metrics` (:164-175).
- **CFC-5: holds.** Behind exits 1 (`hack/ci/go-patch-lag.sh:112`), undetermined exits 2 (:56, :93-96, :104), and a newer minor is only a note (:116). The workflow is schedule plus dispatch (`.github/workflows/go-patch-lag.yaml:8-11`). Stronger than spec: a `go 1.27` directive with no patch number exits 2 (:68), although CFC-1 accepts that form (`consistency_go_toolchain_test.sh:175`).
- **CFC-6: partial.** All five guards have a no-op control, mutants that must fail with a named message, and plain and `--self-test` as separate CI steps (`ci.yaml:554-570, 583-594`). The wiring is checked at `ci_workflow_security_test.sh:599-616`. The required context only exists in `verify-eligibility.sh` so far; the protect-main ruleset is not updated (see first finding).

**What the branch does that no requirement asks for:** it bumps go.mod from 1.26.9 to 1.27.2, along with the devcontainer, golangci-lint (v2.14.0), go-arch-lint (v1.19.0) and govulncheck (v1.9.0). It adds five rows to metrics.md, a new required job, a TCE-3 coverage line in `hack/check.sh`, and new wiring mutants in `ci_docs_gate_test.sh` and `ci_workflow_security_test.sh`.

## Findings
- [WARNING] `tool-pins-selftest` is in `required_checks` but not in GitHub's protect-main ruleset, so CFC-6 "in a required job" is not yet true where merges are actually blocked — `hack/release/verify-eligibility.sh:21`
  Failure: a PR that breaks the tool-pins self-test goes red in that job and can still be merged, because only `verify-eligibility.sh` (the release check) requires it. G6.md:287 records this as gap A11, to be closed by the orchestrator after merge.
  Fix: add the context to protect-main right after this branch merges, and record that it was done.
  Confidence: 90
- [WARNING] The required `lint` job now needs the network: the plain tool-pins step talks to the module proxy, the golangci-lint custom build and the vuln DB — `.github/workflows/ci.yaml:569-570`
  Failure: a proxy or vuln-DB outage turns every PR red, docs-only ones included. That matches CFC-2's "lookup failure fails", but it puts every PR's merge at the mercy of an outside service.
  Fix: accept it and write the trade-off in decisions.md, or add a bounded retry around `go mod download`.
  Confidence: 70
- [NOTE] A tenant exception whose reason is `cluster-scoped` with no colon crashes the Python check (IndexError) instead of printing a clear FAIL. It still fails closed — `hack/test/consistency_rbac_test.sh:186`
  Fix: check `":" in reason` before indexing `[1]`.
  Confidence: 85
- [NOTE] The metrics gate counts any `kollect_*` string literal in `internal/metrics` as registered, not only metric `Name` fields — `hack/test/consistency_metrics_test.sh:139`
  Failure: a label value or log key spelled `kollect_x` would count as registered and would then be required in metrics.md. Nothing in the current tree triggers this.
  Fix: collect only literals or consts used as `Name:` in `*Opts{}`.
  Confidence: 55
- [NOTE] Only `HistogramOpts` names get `_sum` and `_count` series. A future `SummaryOpts` metric queried with `_sum` or `_count` would fail even though the series exists — `consistency_metrics_test.sh:152-161`
  Fix: treat SummaryOpts the same way, for `_sum` and `_count` only.
  Confidence: 60

## Could not check
- I did not run any gate or self-test. Results come from the recorded evidence (G6.md sensor tables).
- The Renovate grouping working at runtime: I only checked the config shape. G1.md says a dry-run confirmed it; I did not read that dry-run.
- The GitHub protect-main ruleset contents, and a first CI run on the PR head.
- The `reviews/B-branch/` directory (untracked, may belong to other reviewers), plus the earlier review legs, which I left alone on purpose.
