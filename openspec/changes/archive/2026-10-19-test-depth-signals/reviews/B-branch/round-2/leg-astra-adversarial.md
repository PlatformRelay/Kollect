## Verdict

BLOCK — the runner fixes hold, but README parsing and execution-wiring guards still accept changes that bypass their requirements.

## Answers

### 1. Closure

“Closed” below means the reported defect is addressed in code; full self-test execution was unavailable under the read-only constraint.

| Item | Closed/open | Closing line | Killing case |
|---|---|---|---|
| A1: later `TYPE/NAME` operands | **Partly closed** | `hack/test/readme_quickstart_test.sh:189` checks all slash-bearing operands; `:196` still truncates before splitting commas | X15 rejects `kinv/a foo/b`; X16 rejects mixed operands. Neither covers the remaining bypasses in R2-A1. |
| A2: `if` / `continue-on-error` | **Closed for the reported fields** | `hack/test/fuzz_retry_test.sh:360`, `:363`, `:366`, `:390`; `hack/test/test_shuffle_test.sh:458` | Runner/job/cache/download conditional and soft-failure mutants; shuffle job/run/checkout mutants. Broader execution controls remain open, R2-A2. |
| A3: retry only exit 1 | **Closed** | `hack/ci/fuzz-retry.sh:90` | F17, killed by “any non-zero exit retried” at `hack/test/fuzz_retry_test.sh:492`. |
| A4: failed Ginkgo invocation | **Closed** | `hack/test/test_shuffle_test.sh:351` injects failure; `:356`–`:360` assert failure and subsequent calls | S13; “a failing Ginkgo run ignored” and “Ginkgo runs stop at the first failure”. |
| O1: fence-language scope | **Closed** | Spec `openspec/changes/test-depth-signals/specs/test-depth/spec.md:168`; guard `hack/test/readme_quickstart_test.sh:125` | X23, a tilde YAML fence; X13, an indented shell fence. |
| Deferred: F16 download conditional/soft failure | **Closed** | `hack/test/fuzz_retry_test.sh:390` | “download step continue-on-error” and “download step if”. |
| Deferred: envtest setup failure lacks annotation | **Closed** | `hack/ci/test-shuffle.sh:47` | S9c; “setup-envtest failure without an error line”. |
| Deferred: empty-package F0 and crash fixture | **Closed** | Empty-package case at `hack/test/fuzz_retry_test.sh:219`; runner rejection at `hack/ci/fuzz-retry.sh:32`; F18 replays the crash fixture | “empty pkg accepted” and “deny markers matched at line start only”. |
| Deferred: continuation/indented-fence fixes unreviewed | **Closed by inspection** | `hack/test/readme_quickstart_test.sh:123`, `:125` | X12, X14 and X13 respectively. |
| Deferred: internal tasks counted as runnable | **Closed for repository Taskfile shapes** | `hack/test/readme_quickstart_test.sh:63`, `:71`, `:85` | X19 internal task, X20 internal include; X21 additionally rejects an empty include path. |

### 2. A1 depth

Yes. Executing the actual parser functions against injected command lines produced:

| Command form | Guard result |
|---|---|
| `kubectl get kinv/a foo/b` | Rejects `foo` |
| `kubectl get kinv/a,foo/b` | **Passes** |
| `kubectl get kinv,foo/b` | Rejects `foo` |
| `kubectl get -f kinv` | **Passes**, treating the filename as a resource |
| `kubectl get --all` | Rejects missing resource |
| `kubectl get kinv --all` | Passes |
| `kubectl get kinv.kollect.dev/a` | Rejects even the qualified Kollect type |
| `kubectl get pods.v1/a` | Rejects |
| `kubectl get kinv/` | **Passes**, without validating the empty name |
| `kubectl get "kinv"` | Rejects the literal quotes |
| `kubectl "get" pods` or `"kubectl" get pods` | **Passes without checking `pods`** |
| `kubectl get --context kinv pods` | **Passes**, mistaking the context value for the resource |
| `kubectl get -n kinv pods` | Correctly rejects `pods` |

The quoted-command and `--context` cases demonstrate valid shell commands naming non-Kollect resources that escape validation.

### 3. A2 depth

The reported `if` and `continue-on-error` holes are closed. Other controls remain unchecked: in-memory probes of F11/F16 and S11 accepted job-level `defaults.run.shell: bash -c "true" {0}` for **both jobs**, allowing their run steps to succeed without execution. Fuzz also accepts changed `needs`, matrix/exclude content and `timeout-minutes: 0`; shuffle rejects job timeout 0 because it pins 45, but accepts an added strategy and does not check step timeouts or shells. Fuzz target coverage is not pinned, so removing a matrix entry escapes these checks. Shuffle’s exactly-one-job assertion limits a valid dependency-based bypass: a nonempty `needs` referring to another job would require violating that assertion. The existing workflow-root allowlist protects top-level defaults in `ci.yaml`, but does not protect either affected job’s defaults.

### 4. A3 depth

The exit-status fix works. With the actual runner body and memory-backed capture, deadline output followed by a passing attempt yielded success and one warning for exit 1; statuses 2 and 137 failed immediately without warning. `PIPESTATUS[0]` at `hack/ci/fuzz-retry.sh:67` captures `go`’s status immediately after the pipeline, rather than `tee`’s. Local Go source also sets the command status to 1 for failed test execution. A test subprocess’s exit code is not necessarily propagated unchanged by `go test`; therefore the new status check supplements the exact-output and deny-marker checks. I found no additional retry bypass or rejection of the captured real flake.

### 5. A4 depth

The runner preserves failures and continues: `hack/ci/test-shuffle.sh:74` and `:79` accumulate `rc=1`, and `:82` rejects the final result. Memory-only probes failing each of calls 1–4 independently always produced four calls and exit 1. The committed fake now proves both failure propagation and continuation for the first Ginkgo failure: S13 checks four invocations and recorded Ginkgo statuses `1,0,0`. Its two mutants separately test swallowed failure and early exit. It does not exhaustively inject failure at every position, but the implementation passed those additional probes.

### 6. Newly broken

G5 introduced a concrete parser regression: previously comma splitting preceded removal of `/NAME`; now `:196` removes everything after the first slash before splitting commas. Consequently `kinv/a,foo/b`, previously rejected for `foo`, passes. I found no newly broken literal mutant anchor, success-message consumer, or misreading of a Taskfile shape currently used by this repository. The fence amendment matches the implemented restriction; the broader TDS-3 claim that resource names are checked remains overstated because of R2-A1.

### 7. Whole-branch review

An additional execution bypass exists in S12: it checks `shuffle.cmds`, but accepts `status: ["true"]` or `ignore_error: true` on the task. Both mutations passed the extracted S12 assertions. The former can skip execution; the latter can hide runner failure. The counter changes retain exact increment assertions of 1 and 1024, and the order-dependent fixture has a real failing ordering; neither is tautological. Reporter-list changes and guard registration appear consistent.

## Findings

### R2-A1 CRITICAL: README tokenization accepts unchecked non-Kollect commands
- Where: `hack/test/readme_quickstart_test.sh:151`, `:176`, `:190`, `:196`
- Evidence: `read -ra w` preserves shell quotes; unknown flags are discarded by `-*) ;;`; only the first unslashed operand is checked. Both `kubectl "get" pods` and `kubectl get --context kinv pods` passed the actual parser. G5 also changed comma/slash processing so `kinv/a,foo/b` passes.
- Why it matters: The guard reports that the README names only Kollect resources while overlooking commands that request `pods`.
- Opposite argument: This is a deliberately limited static checker; however, unsupported syntax must fail closed rather than produce the claimed validation.
- Fix: Tokenize supported shell syntax without evaluating it; reject unsupported forms and unknown flags, correctly consume supported flag values, and split comma-separated operands before validating each type/name pair. Add these bypasses and quoted valid controls to self-tests.

### R2-A2 CRITICAL: Workflow guards permit shells that never execute either signal
- Where: `hack/test/fuzz_retry_test.sh:336`; `hack/test/test_shuffle_test.sh:418`
- Evidence: Both wiring checks accepted a job-level `defaults.run.shell: bash -c "true" {0}` while retaining the required `run` strings.
- Why it matters: GitHub can report successful steps without running module download, fuzzing or shuffle tests. Matrix, dependency and strategy fields also lack the intended execution-integrity checks.
- Opposite argument: Existing workflow-security guards protect execution elsewhere; their closed job-shape checks apply to workflow-security/lint, not these jobs.
- Fix: Add reviewed job/step key allowlists and permitted shell values. Pin fuzz dependencies and target/package matrix coverage, and reject unreviewed shuffle strategies/defaults. Add shell-bypass mutants for both jobs.

### R2-A3 CRITICAL: Task wiring checks accept skipped or ignored shuffle execution
- Where: `hack/test/test_shuffle_test.sh:469`
- Evidence: S12 compares only `.tasks.shuffle.cmds` with `["bash hack/ci/test-shuffle.sh"]`; adding either `status: ["true"]` or `ignore_error: true` passed its assertions.
- Why it matters: `task test:shuffle` can report success without executing the runner or despite its failure, defeating TDS-2 while the runner’s direct tests remain green.
- Opposite argument: The committed Taskfile currently executes correctly; nevertheless, S12 claims to guard that execution and accepts ordinary Task settings that neutralize it.
- Fix: Restrict the shuffle task to its reviewed keys and guard its include binding. Add separate skipped-task and ignored-failure mutants.

## Could not check

- Full plain fuzz/shuffle guards and all `--self-test` suites: they create temporary files, forbidden here.
- Live GitHub scheduling, matrix expansion, CI results, cluster behavior, or fixture provenance through the network.
- Actual kubectl interpretation of every malformed comma/slash operand.
- Real Go/envtest execution; local Go source inspected was 1.27.1, while fixture provenance names 1.27.2.

The real README plain guard passed. Targeted probes used extracted source and anonymous memory-backed files; no repository or `/tmp` files were changed.
