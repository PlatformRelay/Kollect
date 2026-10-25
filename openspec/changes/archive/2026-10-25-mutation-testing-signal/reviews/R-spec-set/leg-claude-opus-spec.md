## Verdict: CONCERNS

## Findings
- [WARNING] MUT-2 promises "no threshold set by any source gremlins reads from the repository", but it only bans `.gremlins.yaml` and `.gremlins.yml`, and T2 only tests those two names. gremlins reads more than that — `specs/mutation-testing/spec.md:50-55`, `tasks.md:116-118`
  Failure: gremlins sets `SetConfigName(".gremlins")` and `SetConfigType("yaml")` (`gremlins@v0.6.0/internal/configuration/configuration.go:74-75`). Viper searches every `SupportedExts` entry, `json` first (`viper@v1.21.0/viper.go:304`, `file.go:77-88`), plus a bare `.gremlins` file, and parses whatever it finds as YAML. A tracked `.gremlins.json` holding `{"unleash":{"threshold":{"efficacy":100}}}` is valid YAML. It sets a gate, and the guard passes it.
  Fix: in MUT-2 and T2, ban every tracked file matching `.gremlins` or `.gremlins.*`, and add a `.gremlins.json` mutant.
  Confidence: 90
- [WARNING] The command-line scan looks for the literal text `--threshold-`. gremlins also accepts the threshold flag spelled with `_` or `.` — `specs/mutation-testing/spec.md:52`, `tasks.md:114-115`
  Failure: the flag normalizer rewrites `.` and `_` to `-` (`cmd/unleash.go:198-206`), so `--threshold_efficacy=100` or `--threshold.efficacy=100` reaches gremlins as the real flag. R1's exact-argv check catches this in the runner. W1 and W2 don't stop the workflow or the Taskfile from calling gremlins directly, and T1 is a text scan, so a call there passes.
  Fix: match `--threshold[-_.]` in the guard (case-insensitive), add a `--threshold_efficacy` mutant, or make W1/W2 forbid any `gremlins` call outside `hack/ci/mutation.sh`.
  Confidence: 75
- [WARNING] The env-var source list leaves out Taskfile `dotenv:` — `specs/mutation-testing/spec.md:56-59`, `tasks.md:119-122`
  Failure: with `dotenv: ['.ci.env']` in `Taskfile.yml` and a tracked `.ci.env` containing `GREMLINS_UNLEASH_THRESHOLD_EFFICACY=100`, gremlins reads the variable through `AutomaticEnv` (`configuration.go:72-73`). Neither scanned file contains the variable name, so the guard passes.
  Fix: in MUT-2 and W2, forbid `dotenv:` in `Taskfile.yml` (or scan the files it names), with a mutant.
  Confidence: 70
- [WARNING] MUT-1's "no other file SHALL name a gremlins version" is enforced only under `.github/`, `hack/ci/` and `hack/task/` — `specs/mutation-testing/spec.md:17-18,43-45`, `tasks.md:110-111`
  Failure: a `go install …/gremlins@v0.5.1` in `Makefile` or in a root-level `hack/*.sh` script (`hack/install-*.sh` live there) passes W2. CFC-2 also passes it, because it only checks that a pin resolves, not that it matches the other pins (`consistency_tool_pins_test.sh:155-188`).
  Fix: scan the same file set CFC-2 scans (Makefile, Taskfile.yml, `.github/`, `hack/` minus `hack/test`) and allow `{{.GREMLINS_VERSION}}` only in the root Taskfile.
  Confidence: 80
- [NOTE] The T3 example `GREMLINS_UNLEASH_THRESHOLD_MCOVER` is not a variable gremlins reads. The env name comes from the config key `unleash.threshold.mutant-coverage`, so the real variable is `GREMLINS_UNLEASH_THRESHOLD_MUTANT_COVERAGE` (`configuration.go:49,70`, `flags.go:66`) — `tasks.md:120`
  Failure: the `GREMLINS_` + `THRESHOLD` pattern still catches it, but the mutant tests a gate that doesn't exist.
  Fix: use `GREMLINS_UNLEASH_THRESHOLD_MUTANT_COVERAGE` in the `$GITHUB_ENV` mutant.
  Confidence: 85
- [NOTE] Default-on mutators can be switched off without the guard noticing. `GREMLINS_MUTANTS_<TYPE>_ENABLED=false` in the env, or `GREMLINS_UNLEASH_EXCLUDE_FILES`, narrows the run, and R1 passes explicit flags only for the default-off set — `tasks.md:80-84`
  Failure: this narrows the measurement rather than gating it, so no requirement is broken. MUT-1's goal of "every mutator" still holds only by convention.
  Fix: either pass every mutator flag explicitly, or name this as a non-goal next to the guard-scope risk.
  Confidence: 60
- [NOTE] M4 (zero mutations) can't happen with v0.6.0. With no mutants, `report.Do` returns nil before `fileReport`, so gremlins writes no file and exits 0 (`internal/report/report.go:255-260`). R3 (missing report) is the case that actually covers it — `tasks.md:65-66`
  Fix: none needed. Note in the 1.1 evidence that zero mutants shows up as "no report".
  Confidence: 85
- [NOTE] Nothing tests MUT-2's "lint SHALL NOT install gremlins" — `specs/mutation-testing/spec.md:62`
  Fix: add a W-clause: no step of the ci.yaml `lint` job mentions `gremlins` outside the guard path.
  Confidence: 55

Checked and holding:
- **MUT-1:** the six default-off mutator names match `mutantenabled.go:28-34` and the flag names in `unleash.go:234-251`. The JSON has `go_module`, `files[].file_name` and `mutations[].status`, and no package path (`structure.go:21-47`). `file_name` is relative to the package directory (`engine.go:72,124-126`). Each package has exactly one non-test file, so M6 can work. The actions and their SHAs are already used in the repo. The renovate regex matches the planned shape (`renovate.json:32`). CFC-2 resolves a `{{.VAR}}` pin only in the root Taskfile (`:163`).
- **MUT-3/MUT-4:** the status strings match (`mutator.go:45-63`). The control LIVED vs NOT COVERED split holds. `--threshold-efficacy 100` exits non-zero even at 100% efficacy (`report.go:237`). A cancelled run exits 0 with no report, and R3 covers that.
- **Line references:** `ci-failure-report.yaml:22-25,34`, `ci_failure_report_test.sh:426-427,1549-1556`, `ci_workflow_security_test.sh:20,432,608` and `hack/check.sh:71-111` all resolve.

## Could not check
- I ran nothing (no gremlins, no guards). All exit codes and statuses above come from reading the source.
- The `reviews/` directory (other legs), `agent-context/decisions.md`, and the change 7/#469 merge state.
- Whether `.github/actions/go-cache` or the DTP guards put further constraints on the new workflow (for example, the `setup-task` `version:` input that W1 doesn't mention).
