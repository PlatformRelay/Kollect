Verdict: REQUEST CHANGES

### DTP-R1 · WARNING · Installer tests lack a success control and an explicit execution home

- Where: `openspec/changes/developer-toolchain-pins/tasks.md:89-99`; `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md:141-145`; `hack/install-git-cliff.sh:43-45`; `hack/install-helm-docs.sh:41-43`; `hack/test/ci_fetch_lib_hardening_test.sh:778-782`.
- Evidence: Task 4.2 specifies only a mismatched digest and a failed checksum fetch, both expecting failure. It names the existing fetch guard for the static scan but gives no file or invocation for the installer execution tests. DTP-7 says the DTP-6 checks “SHALL live in `hack/test/consistency_toolchain_pins_test.sh`.” The existing checksum scan merely counts `verify_sha256|sha256sum` tokens.
- Why it matters: An installer that always fails—for example, because checksum verification uses the upstream asset filename while the downloaded archive is named `git-cliff.tgz`—passes both negative cases. The tasks also leave unclear how the behavioral tests become persistent required checks.
- Proposed fix: Add to task 4.2: “Put both installers’ executable fixture tests in the new guard’s self-test path. For each installer, first prove that a valid archive and matching published-format checksum install the expected executable successfully. Derive the tampered and missing-checksum cases from that passing fixture, assert the intended diagnostic and absent output, and prove that removing verification makes the tampered case fail its assertion.” Explicitly state whether the new guard implements or delegates the static installer scan.

### DTP-R2 · WARNING · The comment rule has no defined attachment or matching grammar

- Where: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md:88-89`; `openspec/changes/developer-toolchain-pins/tasks.md:22-24`; `mise.toml:51-63`.
- Evidence: The requirement exempts “the pin they sit on,” and the positive fixture says only “when a comment names the pin it sits on, then green.” Existing comments include `node-version: "22"` at `mise.toml:57`, followed by a continuation comment before `node = "22"` at line 59; Python similarly has a two-line comment block.
- Why it matters: Neither adjacency, tool identity nor value equality is defined. A checker could reject the existing Node comment because it is not immediately adjacent, or accept stale text merely because it is near a pin. Taskfile comments are also in scope, but the specified comment mutants exercise only mise.
- Proposed fix: Replace the exception with: “A version-bearing comment is permitted only in the contiguous comment block immediately preceding, or inline with, its owning pin; it must identify that tool and equal the pin’s normalized value. Other tool-version restatements and numeric pin-site counts are forbidden.” Define recognized tool/version and count forms. Add positive fixtures for the existing Node/Python blocks and negative fixtures for a mismatched attached version, an unrelated tool version, and a Taskfile comment.

### DTP-R3 · WARNING · DTP-2 tests do not prove that vulncheck uses the named variable

- Where: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md:33-35`; `openspec/changes/developer-toolchain-pins/tasks.md:15-17`; `Taskfile.yml:204`; `hack/test/consistency_tool_pins_test.sh:159-162`.
- Evidence: DTP-2 requires that `task vulncheck` “SHALL use” `GOVULNCHECK_VERSION`. Task 1.1 tests an inline literal and a missing annotation only. CFC-2 resolves any Taskfile variable matching `{{.([A-Za-z0-9_]+)}}`; it does not enforce this variable’s identity.
- Why it matters: Leaving an annotated `GOVULNCHECK_VERSION` unused while running govulncheck through another variable containing the same release violates DTP-2, yet neither prescribed mutant exposes that failure. CFC-2 correctly does not own this structural requirement.
- Proposed fix: Add to task 1.1: “With an annotated `GOVULNCHECK_VERSION` still present, change the vulncheck command to reference another variable containing the same version; require failure naming the missing `GOVULNCHECK_VERSION` reference. Also remove the variable definition and require a specific missing-definition failure.”

### DTP-R4 · WARNING · Single-step removal normally trips CWS-6 before CFC-6

- Where: `openspec/changes/developer-toolchain-pins/tasks.md:29-30`; `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md:152-155`; `hack/test/ci_workflow_security_test.sh:586-608`.
- Evidence: The task says, “Show CFC-6 red when either new step is removed.” The existing guard first iterates scripts still invoked by CI and fails with `CWS-6: ... mode is not invoked by any step of a required job` at line 590. CFC-6’s enrolment loop follows at line 600.
- Why it matters: With one new step remaining and both modes recognized, the earlier CWS-6 check terminates execution. Requiring a CFC-6 diagnostic would make correct wiring protection appear to fail the acceptance test and could prompt an unnecessary change to change 4.
- Proposed fix: Change the scenario and task to require “the workflow-security guard fails, naming the guard and missing mode.” Add a separate fixture deleting **both** steps and require the CFC-6 enrolment diagnostic. Keep decision C unchanged.

The old-name reference inventory in task 1.2 is complete, including hidden files and the old guard’s own self-wiring references. The CFC-2 mutant re-anchoring plan matches the current implementation. No task explicitly requires provisioning operator accounts, secrets or a GitHub App.

Verified file:line claims:

- `mise.toml:8`, `:24`, `:28`, `:39`, `:52`, `:54`.
- `Taskfile.yml:22`, `:31`, `:204`.
- `.github/workflows/ci.yaml:164`, `:299`, `:411-419`.
- `renovate.json:37-47`, `:48-58`, `:74`.
- `Makefile:184`; `hack/tooling/.custom-gcl.yml:6`; `.pre-commit-config.yaml:5`; `hack/install-gitleaks.sh:14`.
- `hack/install-git-cliff.sh:40-43`; `hack/install-helm-docs.sh:40`.
- `hack/test/ci_fetch_lib_hardening_test.sh:766`, `:779`, `:793`, `:844`.
- `hack/test/ci_install_helm_hardening_test.sh:687`.
- `hack/test/ci_workflow_security_test.sh:1145-1146`.
- `hack/test/consistency_tool_pins_test.sh:159-162`, `:499-505`, `:523-529`.
- `.gitignore:26`; `docs/development/setup.md:53`.
- The **18 setup-task sites** count: 17 workflow inputs and one composite-action input, all currently `3.51.1`, plus `mise.toml:54`.
