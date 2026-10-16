# R spec-set review: verification

Reviewed revision: 9621c98aa (spec set from 649e5811c). Legs: astra spec (REQUEST CHANGES),
claude spec (CONCERNS). Every WARNING below was checked against the tree by the orchestrator.

| ID | Sev | Legs | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| R1 | WARNING | claude | yes | A live shellcheck manager bumps `ci.yaml:299` only; `hack/install-shellcheck.sh:42-68` has digests for 0.11.0 alone, so every bump is red and holds the shared "pinned build tools" group; `install-shellcheck.sh:15` and `.pre-commit-config.yaml:56` drift | fix |
| R2 | WARNING | claude | yes | Header rule "red check lands with the behaviour task" cannot hold for 1.1, which is red for 2.1, 2.2 and 2.3 | fix |
| R3 | WARNING | claude, astra | yes | DTP-7 puts DTP-6's checks in the new guard, but the installer scan lives in `ci_fetch_lib_hardening_test.sh:778-782` and 4.2 names no home for the tamper tests | fix |
| R4 | WARNING | claude | yes | The "only shrinks" ceiling for `UNVERIFIED_BY_DESIGN` is never tightened to empty in 4.3 | fix |
| R5 | WARNING | claude | yes | DTP-4 extraction and grouping checks have no stated mechanism (Renovate in lint vs a bash copy of JS regex and packageRules order) | fix |
| R6 | WARNING | astra | yes | 4.2 has no success control: an installer that always fails passes both negative cases | fix |
| R7 | WARNING | astra | yes | DTP-5 "the pin they sit on" has no attachment or matching grammar | fix |
| R8 | WARNING | astra | yes | Removing one step trips CWS-6 at `ci_workflow_security_test.sh:590` (`fail` exits) before CFC-6 at :608 | fix |
| R9 | WARNING | astra | yes | No mutant proves `task vulncheck` references `GOVULNCHECK_VERSION` rather than another variable | fix |
| R10 | WARNING | orchestrator | yes | git-cliff has three pins: `Taskfile.yml:22`, `release.yaml:420` (git-cliff-action `version:`), `hack/verify-changelog.sh:11`; DTP-3/DTP-4 cover only the first | fix |
| R11 | WARNING | orchestrator | yes | `docs/development/setup.md:39` restates Task 3.51.1; 2.1 leaves it stale | fix |
| R12 | NOTE | claude | yes | govulncheck annotation datasource unspecified | fix |
| R13 | NOTE | claude | yes | A new group rule must precede the "go toolchain" rule (`renovate.json:142`) | fix |
| R14 | NOTE | claude | yes | `sha512sum` is absent on stock macOS; `verify-sha256.sh:21` has the same gap | defer |
| R15 | NOTE | claude | yes | `task-prompt.md:51` names the guard 1.2 deletes as the template | fix |
| R16 | NOTE | claude | yes | CFC-2/CFC-6 live only in the unarchived change 4 | reject |
| R17 | NOTE | orchestrator | yes | gitleaks sites differ in `v` prefix (`.pre-commit-config.yaml:5`); 3.1 should prove each form is rewritten correctly | fix |

All other file:line claims in the spec set were re-checked and hold, including the 18 setup-task
sites, the CFC-2 mutant lines and the complete list of `dev_mise_pin_drift` references in 1.2. No
task needs an operator account, a secret or a GitHub App.
