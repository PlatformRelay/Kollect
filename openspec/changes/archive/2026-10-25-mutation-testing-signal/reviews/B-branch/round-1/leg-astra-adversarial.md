## Verdict
verdict: BLOCK
findings:
  - [CRITICAL] hack/ci/mutation-summary.sh:74 accepts a package with zero completed measurements: changing aggregate’s five statuses to TIMED OUT produced exit 0 and counts 0/0/0/5/0. Require a usable per-package measurement or fail diagnostically. — Q2
  - [NOTE] Counterargument: timeouts can legitimately detect hanging mutants and are displayed separately; nevertheless, gremlins@v0.6.0/internal/engine/executor.go:194 starts its deadline before compilation, using 3× coverage elapsed (:101), with CPU-count concurrency (internal/engine/workerpool/workerpool.go:97). Thus infrastructure timeouts can leave an entire package unmeasured and green; the critical survives. — Q2
  - [NOTE] hack/ci/mutation.sh:43 rejects GREMLINS_* configuration, :52 disables test-result caching, :65 creates fresh reports, and :76 propagates failures; Taskfile.yml:221 and .github/workflows/mutation.yaml:42 do not swallow them. No current score gate found. — Q1
  - [NOTE] hack/test/mutation_signal_test.sh:599 enforces workflow shape, permissions, step keys, conditions, env and SHA references; :638 fixes the upload path. Extracted W1 checks passed baseline and rejected in-memory job continue-on-error and empty-upload-path mutations. W3 (:755) remains a textual scan, not arbitrary shell analysis. — Q3
  - [NOTE] Taskfile.yml:32 matches the Renovate annotation pattern; :217 installs into an absolute version-named GOBIN, and :223 invokes that binary after the dependency. Neither task has skip/error-softening settings. — Q4
  - [NOTE] .github/workflows/ci-failure-report.yaml:26 exactly matches mutation.yaml:10; its job filter (:35) admits scheduled runs from this repository, including failures. — Q5
  - [NOTE] hack/ci/mutation.sh:46 permits empty/false and fails for true or invalid values; mutation.yaml:41 passes the variable through env without shell interpolation. Forced failure precedes report-output creation (:65), so the always-upload step receives an empty path and should fail its required-path validation; it cannot make the run green. — Q6
could_not_check:
  - Full guard/self-test and real mutation reruns: sandbox denies /tmp writes. Probes used memory only; the seven reported runs were not independently reproduced.
  - Pinned checkout/upload action implementations unavailable offline; upload conclusions rely on documented input semantics. Scheduled execution, artifact persistence and reporter issue lifecycle remain unverified.
summary: Install and workflow wiring are coherent, but all-timeout package reports can falsely certify a successful measurement.
