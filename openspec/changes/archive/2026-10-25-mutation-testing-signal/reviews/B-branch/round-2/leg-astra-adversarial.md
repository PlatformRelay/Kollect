## Verdict
verdict: BLOCK
findings:
  - [CRITICAL] hack/test/mutation_signal_test.sh:861 — Defense: literal/folded blocks are covered; however, plain `run: task` followed by an indented `mutation` parses as `task mutation` and produces no W3 hit, allowing mutation execution in another workflow. — Q1/Q3
  - [WARNING] hack/test/mutation_signal_test.sh:890 — A legitimate `run: |` containing `task mutation` in mutation.yaml passes W1 but fails W3: its exemption requires the literal text `run: task mutation`. Reproduced in memory. — Q2
  - [NOTE] hack/ci/mutation-summary.sh:80 — B1 closed: every package needs KILLED or LIVED; the recorded all-TIMED-OUT report now fails without a table. Zero killed with survivors remains valid. — Q1/Q2
  - [NOTE] hack/ci/mutation.sh:88 — B2 closed for v0.6.0’s generated help format; the real binary exposes all eleven matching flags, and additions/removals cause inequality. — Q1/Q2
  - [NOTE] hack/test/mutation_signal_test.sh:848 — Original B4 continuation cases are covered here and at :861; the plain-scalar bypass above remains. B5 closes at :281, which consumes the warm-cache fixture. — Q1
  - [NOTE] hack/test/mutation_signal_test.sh:861 — New block-query and jq failures explicitly reach scan_error (:862/:868); no swallowed error found in this added path. — Q2
  - [NOTE] hack/ci/mutation.sh:71 — Coefficient 10 gives each mutant 10× measured coverage duration, not an absolute deadline below 15 minutes; .github/workflows/mutation.yaml:26 supplies the overall job limit. — Q2
  - [NOTE] hack/ci/mutation.sh:98 — Explicit empty config, environment refusal (:42), and nonzero propagation (:99) preserve report-only behavior; no score threshold found. — Q3
  - [NOTE] Taskfile.yml:214 — Install chain remains pinned; reporter includes Mutation testing (.github/workflows/ci-failure-report.yaml:26); MUTATION_FORCE_FAILURE only rejects the run (hack/ci/mutation.sh:49). — Q3
could_not_check:
  - Full guard/self-test and real mutation execution: sandbox rejects temporary-file creation; formatter fixtures, real binary help, and extracted W3 scans were exercised read-only.
  - Hosted-runner timing, installation/downloads, artifact upload, and scheduled failure reporting; network was prohibited.
summary: B1, B2 and B5 close; original B4 examples close, but semantic YAML coverage remains incomplete.
