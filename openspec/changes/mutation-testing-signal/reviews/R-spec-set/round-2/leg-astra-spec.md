## Findings

**A1 — CRITICAL — W2 still permits a successful run without measurement.**

Evidence: `openspec/changes/mutation-testing-signal/tasks.md:131-139` pins commands and excludes `status`, but leaves `sources`, `generates` and `method` unrestricted. The repository already uses source/generated-file skipping (`Taskfile.yml:80-85`). Cached Task source confirms this check is independent of `status`: `/home/koni/go/pkg/mod/github.com/go-task/task/v3@v3.52.0/internal/fingerprint/task.go:95-108` and `task.go:229-246`.

Adding file-based freshness checks to `mutation` can skip the runner, including its missing-report and forced-failure checks, while satisfying W2. The direct-runner tests do not exercise Task.

Smallest fix: forbid freshness controls on both tasks and add a skip mutant. Require scalar command entries, or explicitly reject command-level `ignore_error` too; checking only the task-level key is insufficient.

**A2 — CRITICAL — W3 does not enforce the required-lint exclusion or its claimed scan scope.**

Evidence: `tasks.md:140-145` looks for `go-gremlins` and a `gremlins` command word. A new required-lint step containing `task mutation`, or a dependency from `lint` to `mutation`, contains neither. It installs and executes gremlins while passing those specified checks, contrary to `specs/mutation-testing/spec.md:78-79`.

Furthermore, CFC-2’s actual enumeration at `hack/test/consistency_tool_pins_test.sh:175-188` does not recursively discover included Taskfiles: it scans the root Taskfile, `.github` YAML and files under `hack`. An included Taskfile elsewhere is outside that enumeration. `.github` shell scripts are also outside it.

Smallest fix: explicitly check that required-job commands and their Task dependencies cannot reach the mutation/install tasks; add a `lint: task mutation` mutant. Define and implement the scan set directly, including recursively referenced Taskfiles, rather than calling it “the file set CFC-2 scans.”

**A3 — CRITICAL — Fresh reports have no defined artifact handoff.**

Evidence: `tasks.md:92-93` and `:167-170` require a fresh temporary report directory. W1 at `:126-127` requires an upload action but does not specify its `path`, a run-step output, report retention, or missing-file behavior. The implementation task defines none of these. `specs/mutation-testing/spec.md:40-42` nevertheless requires the report artifact.

An implementation can satisfy all named pre-merge cases while uploading the wrong directory or deleting reports when the runner exits. The first artifact assertion is post-merge (`tasks.md:207`).

Smallest fix: define a runner-to-workflow output carrying the fresh directory, retain it until upload, pin upload’s path to that output, and test that handoff before merge.

**A4 — WARNING — C3 and the YAML cases do not adequately test the new fail-closed promise.**

Evidence: `specs/mutation-testing/spec.md:76-78` requires *every* read and YAML parse to fail closed. C3 (`tasks.md:162-164`) exercises only malformed `mutation.yaml` and failed `git ls-files`. A failed read during W3 enumeration or parsing `Taskfile.yml`, the committed config, or reporter YAML can still be treated as an empty result without these controls detecting it.

W1/W2/T3 also do not prescribe semantic handling or rejection of anchors, flow mappings and quoted keys. Their ordinary-form mutants would permit a line-oriented implementation that misses equivalent settings.

Smallest fix: require semantic YAML validation with duplicate-key rejection and an explicit policy for aliases/merges. Add an unreadable enumerated file and malformed Taskfile/config cases, plus alternate-YAML representations of forbidden settings. Judge each by its own scan-error diagnostic.

**A5 — WARNING — R6 samples three variables but does not establish the blanket refusal.**

Evidence: `tasks.md:109-112` tests only three nonempty variables; `:189` deletes the entire refusal as its sole defect control. A three-name denylist passes these tests but violates the “any `GREMLINS_` variable” requirement at `specs/mutation-testing/spec.md:72-74`. An implementation that ignores empty values also passes, although an empty exported variable is still set.

Smallest fix: add an unrelated `GREMLINS_` name, an empty exported variable, and another narrowing variable such as `GREMLINS_UNLEASH_TAGS`; mutate blanket prefix detection into a fixed denylist. The explicit config and exact argv otherwise close the pinned version’s known config and narrowing-flag routes.

**A6 — WARNING — V12’s task-sizing problem remains.**

Evidence: task 2.3 (`tasks.md:117-145`) still contains well over fifteen independently testable conditions: trigger shape, permissions, environment levels, action order and pins, checkout input, exact command, condition/error/shell exclusions, pin annotation and placement, install command/path, dependency shape, task exclusions and repository-wide scanning. Task 2.2 (`:86-116`) likewise combines numerous distinct runner behaviors.

Smallest fix: split these into bounded test tasks while retaining the single G2 commit. Explicitly enumerate W1/W2 mutants; `tasks.md:190` refers to mutants those cases “list,” but those cases mostly list assertions rather than concrete mutations.

**A7 — NOTE — Round-1 closure assessment.**

Paths below are relative to `openspec/changes/mutation-testing-signal/` unless otherwise stated.

| Finding | Status | Evidence / remainder |
|---|---|---|
| V1 | Closed | `specs/mutation-testing/spec.md:65-68`; `tasks.md:150-153`: bare name, arbitrary extensions and explicit empty config. |
| V2 | Closed | `tasks.md:48-52`: corrected CLI/environment threshold expectations. |
| V3 | Partly closed | Runtime refusal and normalized flag spellings added; scan-error and YAML coverage remain incomplete (A4). |
| V4 | Partly closed | `tasks.md:120-139` closes extra steps, `|| true`, conditions and workflow error suppression; Task skipping remains (A1). |
| V5 | Partly closed | Exact argv plus blanket runtime requirement close the intended paths; R6 does not establish blanket enforcement (A5). |
| V6 | Closed | `tasks.md:66-80`, `:99-101`: validate before printing, fresh reports and stale-report control. |
| V7 | Closed | `tasks.md:68-71`, `:99-101`, `:113-116`, `:182-190`: synthetic statuses, R7 and diagnostic-specific judging. |
| V8 | Closed | `proposal.md:52-57`; `tasks.md:178-181`: module-proxy dependency acknowledged and cold/warm timing required. |
| V9 | Partly closed | Root pin shape corrected; scan scope remains incomplete (A2). |
| V10 | Closed | `specs/mutation-testing/spec.md:19-21` permits provenance/version references. |
| V11 | Closed | `specs/mutation-testing/spec.md:26-31`; `tasks.md:84-85`, `:102-105`: formatter interface, package cardinality and exact control identity. |
| V12 | Not closed | Splitting threshold cases out did not bound the remaining task (A6). |
| V13 | Closed | `tasks.md:155` uses `THRESHOLD_MUTANT_COVERAGE`. |
| V14 | Closed | `tasks.md:77-78` explicitly distinguishes the synthetic format case from current tool behavior. |
| V15 | Partly closed | Direct lint installation mutant added at `tasks.md:145`; indirect invocation remains (A2). |

**A8 — NOTE — Pin shape, reporter edits and existing citations are consistent.**

The prescribed unquoted, two-space root variable and immediately preceding annotation satisfy CFC-2’s resolver (`hack/test/consistency_tool_pins_test.sh:129-132`, `:163-168`) and Renovate’s matcher (`renovate.json:27-36`). The annotation introduces no version-bearing comment conflicting with DTP-5 (`hack/test/consistency_toolchain_pins_test.sh:522`).

The reporter changes at `tasks.md:172-173` correctly cover its exact list, comment, success/failure messages and removal mutant. Existing removal mutants match the shared diagnostic prefix (`hack/test/ci_failure_report_test.sh:424-427`), and its self-test copies all workflow files (`:125`). No additional reporter-case conflict was found.

All existing-tree `path:line` citations in the three documents checked against their stated targets; no stale citation was found. The cached configuration source additionally lists the current directory (`configuration.go:147-148`), omitted from the proposal’s inventory, but explicit `--config` closes that source too.

The intended guard uses fake gremlins, so real execution is not required in lint. CFC-2 adds module resolution, as acknowledged; cold/warm and guard timings are required, but remain unmeasured until implementation.

## Verdict

BLOCK
