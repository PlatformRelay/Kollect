## Findings

**A1 — CRITICAL — T2 omits configuration files that the pinned tool reads.**

Evidence: `specs/mutation-testing/spec.md:50-55` and `tasks.md:116-118` prohibit only `.gremlins.yaml` and `.gremlins.yml`. In the cached gremlins source, `internal/configuration/configuration.go:74-91` sets the config name to `.gremlins` and delegates discovery to Viper. Its pinned dependency, `/home/koni/go/pkg/mod/github.com/spf13/viper@v1.21.0/file.go:75-88`, searches all supported extensions **and the extensionless name**. `viper.go:304` lists those extensions; `viper.go:2032-2034` preserves the explicitly selected YAML decoder.

Consequently, an extensionless `.gremlins`, or YAML content in `.gremlins.json`, can configure numeric thresholds while every T2 mutant passes. The proposal’s claim that another config filename requires a future version is incorrect (`proposal.md:114-119`).

Smallest fix: prohibit every filename the pinned discovery mechanism accepts, including extensionless `.gremlins`, and add corresponding mutants. Also cover repository-controlled configuration written at runtime or selected through config-path settings.

**A2 — CRITICAL — The prescribed threshold probe contradicts the cached source.**

Evidence: `tasks.md:43-45` requires `--threshold-efficacy 100` to produce a nonzero exit. Gremlins registers this as a float64 flag (`cmd/unleash.go:217`), but its `configuration.Get[T]` uses a direct type assertion (`internal/configuration/configuration.go:181-188`). Viper v1.21.0 handles a changed scalar float64 flag through its string-returning fallback (`viper.go:1176-1219`). Gremlins’ float64 and int assertions therefore both produce zero in `internal/report/report.go:233-238`. Automatic environment values likewise arrive as strings (`viper.go:1226-1232`).

Numeric YAML thresholds can gate; the specified CLI probe does not establish that behavior. Task 1.1 explicitly requires stopping when such a contradiction appears.

Smallest fix: record this pinned-version behavior accurately; use a numeric YAML threshold as the positive threshold control. Retain the policy prohibition on CLI and environment thresholds, but distinguish that policy from their current effectiveness. The actual mutant-coverage environment key derives from `unleash.threshold.mutant-coverage`, not `THRESHOLD_MCOVER` (`configuration.go:49,70`).

**A3 — CRITICAL — The static cases do not establish fail-closed parsing or scanning.**

Evidence: `tasks.md:96-126` supplies ordinary YAML forms and literal threshold strings, but no parser contract or scan-error controls. An implementation satisfying those cases can miss quoted keys, flow maps, anchors/aliases, or an expression-valued environment such as `env: ${{ fromJSON(vars.MUTATION_ENV) }}`. Such an expression need not contain the prohibited variable name in repository text.

T1 also tests only `--threshold-efficacy`. Gremlins accepts dots and underscores in flag names by normalizing them to hyphens (`cmd/unleash.go:196-205`). Shell quoting and backslash continuations provide additional representations that literal scans need not recognize.

Neither T2 nor the other cases requires a failed `git ls-files`, YAML parser, or file read to fail the guard. “Scan failed” can therefore become “nothing prohibited found.”

Smallest fix: specify structural YAML validation, explicit rejection of unresolved environment expressions and unsupported forms, normalized command inspection or a strict command allowlist, and fail-closed handling of every scan. Add mutants for alternate representations, nested Taskfile environments, and scanner failures.

**A4 — CRITICAL — W1/W2 allow the actual execution chain to hide failures.**

Evidence: W1 only says the workflow “runs `task mutation`” (`tasks.md:103`); W2 says the task “runs `hack/ci/mutation.sh`” and lacks `status:` or `ignore_error:` (`tasks.md:107-113`). Neither requires an exact executable command or verifies the full Taskfile dependency chain. R1–R5 invoke the runner directly with a fake binary (`tasks.md:76-95`).

Thus `task mutation || true`, `task mutation; exit 0`, or `bash hack/ci/mutation.sh || true` can preserve the named commands and all direct runner tests while making a broken nightly green. An extra workflow command can also implement a score gate. Installing into a versioned directory does not prove the runner uses that binary; the install dependency or PATH wiring can be missing.

Smallest fix: constrain the workflow command and Taskfile execution chain, including command-level error handling, and test that chain with fake installation and runner commands. Assert that failures propagate and that the installed versioned binary is selected.

**A5 — CRITICAL — Exact runner argv does not prevent environment-driven narrowing.**

Evidence: R1 checks arguments, while T3 checks only threshold variables (`tasks.md:80-84,119-122`). Gremlins reads `unleash.diff`, `unleash.tags`, and `unleash.coverpkg` from configuration (`internal/configuration/configuration.go:40-47`), including automatic environment values. The consumers use string values directly (`internal/diff/parse.go:26`; `internal/coverage/coverage.go:71-72`).

A Taskfile or workflow environment can therefore set `GREMLINS_UNLEASH_DIFF`, `GREMLINS_UNLEASH_TAGS`, or `GREMLINS_UNLEASH_COVERPKG` without changing R1’s argv. A narrowed report containing some valid mutants passes M4 and can hide survivors. The fake does not model this environment behavior.

Smallest fix: define and enforce an allowed effective gremlins configuration, including environment sources, and test prohibited narrowing settings at the workflow/task boundary. Do not conflate `--output-statuses` with JSON filtering: this version applies that setting to logging (`internal/report/logger.go:25`; JSON generation is separate in `internal/report/report.go:166-188`).

**A6 — CRITICAL — Fresh reports and atomic summary publication are not required by the cases.**

Evidence: R3 tests a tool that writes nothing, but does not preseed an old valid report (`tasks.md:88-89`). M7 uses a nonzero tool failure for its middle night (`tasks.md:71-75`). Neither catches reuse of stale reports when the tool exits zero without generating a new one. Gremlins itself returns success for no results and for cancellation (`internal/report/report.go:255-260`; `cmd/unleash.go:135-136`).

Additionally, M4–M6 require failure but not an `::error::` annotation and no counts table (`tasks.md:65-70`), although MUT-4 requires both for tool failure (`specs/mutation-testing/spec.md:119-123`). A formatter can publish earlier package rows before encountering an invalid later report.

Smallest fix: require a fresh per-run report directory and add a preseeded-stale-report test. Validate all reports before publishing any table; test each invalid-report class in the last package and assert the required annotation and absence of a partial table.

**A7 — WARNING — Several red-first and defect-control instructions cannot reliably demonstrate the stated defect.**

Evidence:

- M7 belongs to the formatter task but requires a runner that is not stubbed until 2.2 (`tasks.md:55-77`). Its first failure can be a missing file, expressly forbidden at `tasks.md:3-4`.
- R2’s named stub makes only one call, so it never reaches the second-package failure being tested (`tasks.md:76-87`).
- W1 is supposed to fail before the workflow exists, despite the prohibition on missing-file reds (`tasks.md:96-106,132`).
- Removing the runner’s report-exists check need not make R3 pass: the formatter independently rejects missing reports. Removing the formatter’s explicit check can likewise leave JSON/file-reading failures intact (`tasks.md:62-64,88-89,143-146`).
- The “timed out as killed” mutant is not guaranteed observable: no task guarantees that M1’s recorded reports contain a timed-out mutant (`tasks.md:50,57-59,144`).

Smallest fix: use valid, case-specific defective stubs; move M7 after runner scaffolding; provide synthetic fixtures covering every counted status; and choose behavioral mutations that survive redundant checks unless the intended assertion catches them.

**A8 — WARNING — The required-job proxy claim is false, although gremlins is not installed there by CFC-2.**

Evidence: `proposal.md:42-44` promises no new required-check dependency on the module proxy. However, required `lint` already invokes CFC-2 at `.github/workflows/ci.yaml:619-621`. CFC-2 scans the new Taskfile pin and calls `go mod download` while resolving it (`hack/test/consistency_tool_pins_test.sh:163-170,209,346-357`). Gremlins is outside the analysis-tool smoke list (`:319-325,391-395`), so this introduces module resolution, not gremlins installation or execution.

Task 2.4 does request guard timings and the lint delta (`tasks.md:141-142`); that part is covered. It does not define a runtime budget.

Smallest fix: acknowledge the new resolution dependency explicitly and include cold-cache CFC-2 resolution in the measured delta. Keep the new guard hermetic, with fake gremlins only, and set an acceptable runtime budget if “fast lint” is an acceptance condition.

**A9 — CRITICAL — W2 does not enforce the installation/version scope promised by MUT-1.**

Evidence: MUT-1 prohibits version restatements in workflows, included Taskfiles, and scripts (`specs/mutation-testing/spec.md:41-45`). W2 scans only `.github/`, `hack/ci/`, and `hack/task/` for `gremlins@` (`tasks.md:111`). A differently located included Taskfile or script can escape that scan. Unversioned downloads or wrappers can also install gremlins without containing `gremlins@`.

Furthermore, merely putting the variable somewhere in root `Taskfile.yml` is insufficient for CFC-2: its resolver expects a two-space literal entry (`hack/test/consistency_tool_pins_test.sh:129-132`). The intended top-level `vars:` placement should be explicit.

Smallest fix: prescribe the top-level literal pin shape and inspect all executable install paths reachable from `task mutation`, with one allowed installer. Add controls for another root-level installer, an included Taskfile outside `hack/task/`, and a download-based installer.

**A10 — WARNING — The single-version rule contradicts required provenance documentation.**

Evidence: `specs/mutation-testing/spec.md:17-18` says “no other file SHALL name a gremlins version.” Yet `tasks.md:17-18` requires fixture README files to name the gremlins version, and all three spec-set documents already name v0.6.0.

Smallest fix: restrict the single-source rule to executable installation/configuration pins. Explicitly permit historical evidence and fixture provenance.

**A11 — WARNING — The formatter and fixture-control interfaces remain underspecified.**

Evidence: `tasks.md:56` introduces `<pkg>=<report>...`; `tasks.md:130` specifies `$GITHUB_STEP_SUMMARY`. Neither interface is fixed in the spec. The cases do not explicitly reject duplicate or missing package arguments, malformed JSON structure containing otherwise valid statuses, or an unrelated LIVED mutant standing in for an absent control. Task 1.1 records file, line, and type (`tasks.md:47-49`), but does not define the control’s identity or matching cardinality; the report also supplies a column (`internal/report/internal/structure.go:43-47`).

Smallest fix: define the formatter’s argument set, schema validation, output/error channels, and exact control identity. Add duplicate/missing-package tests and a fixture report where the target control is absent but another mutant is LIVED.

**A12 — WARNING — Task 2.3 exceeds the requested reviewable task size.**

Evidence: `tasks.md:98-126` bundles well over fifteen independently testable claims: workflow triggers, permissions, actions, timeout, credentials, execution, failure knob, conditionals, shell configuration, installation, versioning, threshold sources, reporter wiring, and controls. Task 2.5 then asks for a mutant per clause without enumerating those mutants (`tasks.md:143-149`).

Smallest fix: split 2.3 into workflow structure, Taskfile/install chain, configuration rejection, and reporter wiring tasks. Enumerate each mutation and expected diagnostic while retaining the single green G2 commit.

**A13 — NOTE — Reporter edits, cited locations, and the intended pin annotation are consistent with the current tree.**

Evidence: the current reporter list is exactly `E2E nightly`, `Go patch lag`, `Test shuffle` (`.github/workflows/ci-failure-report.yaml:22-25`; `hack/test/ci_failure_report_test.sh:1549-1556`). Task 2.4 explicitly updates the exact assertion, comment, message, and drop-entry mutant. Existing reporter step-order cases concern its unchanged three-step job (`hack/test/ci_failure_report_test.sh:475-482`), and its self-test copies the workflow directory (`:125`).

The cited `path:line` locations in the three documents match the tree. The intended unquoted, two-space `vars:` pin and adjacent annotation fit CFC-2, DTP-5, and `renovate.json:27-36`.

Smallest fix: none for these points; preserve these edits and run the specified existing guards after implementation.

## Verdict

BLOCK
