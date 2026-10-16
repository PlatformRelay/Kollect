## Verdict: CONCERNS

## Findings
- [WARNING] Task 7.2's rule says "any `$` fails", but it exempts every `golang:`-prefixed image. A variable placed after the literal version passes as long as it carries a digest. — `hack/test/consistency_go_toolchain_test.sh:95-96,187`
  Failure: `FROM golang:1.27.2@${D}` or `FROM golang:1.27.2-${X}` with `--build-arg D=sha256:<digest of golang:1.99.0>` (or `X=alpine@sha256:…`). Line 96 strips `@…`, `ver="${tag%%-*}"` gives `1.27.2`, and the gate goes green. Docker pulls by the digest and ignores the tag, so the image ships Go 1.99. This is the "only known at build time" class that 7.2 and the header comment (:15-17) claim to close. No mutant covers it. The round-2 fix `9203283bf` was never independently reviewed (G7.md:208).
  Fix: drop the `img !~ /^golang:/` exemption so any `$` reports `<variable>`, and update the `golang:${GO_VERSION}` mutant's expected message. Or reject a `$` after `golang:` explicitly, and add a `golang:1.27.2@${D}` mutant.
  Confidence: 75
- [WARNING] CFC-1 is only partly met for registry-qualified golang images. Only the `docker.io/` and `library/` prefixes are stripped. — `hack/test/consistency_go_toolchain_test.sh:90,96`
  Failure: a Dockerfile with `FROM mirror.gcr.io/library/golang:1.99.0 AS builder` (or `localhost:5000/golang:1.99.0`) prints no image line. The devcontainer still supplies one, so the gate passes while the release builds with 1.99. The spec says "every `Dockerfile*` that builds Go". G7.md:211-214 records this as pre-existing and sends it to the BACKLOG, so CFC-1 is not fully met on this branch.
  Fix: match `(^|/)golang:` after the last `/`-separated path segment, plus a mutant. Or state the limit in the spec.
  Confidence: 85
- [WARNING] CFC-6 is still partial: `tool-pins-selftest` is required only in `verify-eligibility.sh`, not in the protect-main ruleset (round-1 O1, deferred). — `hack/release/verify-eligibility.sh:21`, `.github/workflows/ci.yaml` job `tool-pins-selftest`
  Failure: a PR that breaks the tool-pins self-test goes red in that job and can still be merged.
  Fix: the post-merge owner row (add the context to protect-main). Keep it visible in the PR description.
  Confidence: 85
- [NOTE] Stronger than the spec: the `$` rule covers every `Dockerfile*`, not only ones that build Go. — `hack/test/consistency_go_toolchain_test.sh:95`
  Failure: a future non-Go Dockerfile, or a runtime stage using `FROM gcr.io/distroless/static:${TAG}`, goes red even though CFC-1 does not apply to it. Nothing in today's tree does this (G7.md:49). The header documents the choice (:15-17).
  Fix: accept it and record it in decisions.md, or limit the rule to files that have a golang stage.
  Confidence: 60
- [NOTE] G7.md contradicts itself: :48 says `debian:${DEB_TAG}` "is now a mutant", while :50-51 still says it "passes (green control)". The code and the mutant at `consistency_go_toolchain_test.sh:470-472` match :48. — `openspec/changes/cross-file-consistency-gates/evidence/G7.md:50-51`
  Fix: delete the stale sentence.
  Confidence: 95
- [NOTE] The Verification table in tasks.md is still all `not-run`, even though G1–G7 record passes. The `all` row (lint job on the PR head) really has not run yet. — `openspec/changes/cross-file-consistency-gates/tasks.md:44-51`
  Fix: fill in Status/Evidence before merge, or say that it happens at landing.
  Confidence: 70

**Round-1 fixes verified (by reading the code)**
- 7.2: holds for `${B}`, `$B`, `${B:-…}`, `localhost:5000/${B}:…`, `localhost:${B}` and `debian:${…}`, at :95 and :182-186. `<variable>` is left out of the disagreement set at :210. The gap is the first finding above.
- 7.3: holds. `registerTo` split at `metrics.go:303-342`. The Describe-based name comparison is at `metrics_catalog_test.go:102-143` and the controller-runtime registry proof at :159-185. It fails closed on a pointer-typed labeled collector (the `n == 0` check). The shared `sync.Once` stops the double-`Register()` panic.
- 7.4: holds. `new_copy` runs in the parent shell (:327-334), so `SCRATCH` accumulates for the EXIT trap (:312).
- 7.5: holds. `partition` at `consistency_rbac_test.sh:186-191`, with a mutant at :393-397.

**CFC compliance on the whole branch**
- CFC-1: partial (the two WARNINGs above). go.mod, `toolchain` line, setup-go, Renovate order and the empty-scan checks hold (:163-293).
- CFC-2: holds. `GOTOOLCHAIN=go<ver>` at `consistency_tool_pins_test.sh:332`, and Taskfile.yml:14 sets it too.
- CFC-3: holds. No change since round 1 apart from 7.5.
- CFC-4: holds, and is stronger than spec now that the test compares `Register()` itself.
- CFC-5: holds. Behind exits 1 at `go-patch-lag.sh:112`, an undetermined patch exits 2 at :56/:93-96, and a newer minor is only a note at :116. Weekly cron with `contents: read`.
- CFC-6: partial (protect-main). Plain and `--self-test` run as separate steps at `ci.yaml:554-570`, and the self-test has its own job.

**Not in any requirement:** the Go 1.27.2 bump, the tool-version bumps, five new metrics.md rows, the `registerTo` refactor in product code, and the `TestRegister` change to `registerOnce`.

## Could not check
- I ran no gate, self-test or Go test. Results come from G7.md's sensor tables, so the digest bypass is shown by reading the code only.
- GitHub's protect-main ruleset and a CI run on the PR head.
- The full bodies of `consistency_metrics_test.sh`, `consistency_rbac_test.sh` and `consistency_tool_pins_test.sh` outside the changed hunks; for those I relied on the round-1 spec leg's line references. Also `ci_workflow_security_test.sh` and `ci_docs_gate_test.sh`.
- The untracked `reviews/B-branch/round-2/` (other reviewers' output, left alone on purpose).
