# Stage B round 1: verification of leg findings

Verified by the B orchestrator against HEAD `8383c5995`. Line numbers are at that commit.

| Id | Sev | Leg | Verified? | Finding | Proposed disposition |
|---|---|---|---|---|---|
| B1 | P1 / WARNING | astra | yes, reproduced | `hack/test/consistency_toolchain_pins_test.sh:1285`: the `git-cliff with: before uses:` sentinel hard-codes `v2\.13\.1`, so a correct Renovate git-cliff bump reds `--self-test` (holistic.md bump simulation); the shared "pinned build tools" group goes red with it. Same class: `:1268` absence probe `!^ version: v2\.13\.1` turns vacuous after a bump, and `:1277-1280` insert a literal `v2.13.1` | fix: derive the current git-cliff version from `Taskfile.yml` (as `SC_CUR` does at `:1293`) and use it in `:1268`, `:1277-1280`, `:1285`; add a self-test bump control |
| B2 | WARNING | claude/opus:spec | yes | `.github/workflows/ci.yaml:570` runs the new `--self-test` (233s, `evidence/G4.md:161`) in `lint`, while `ci_workflow_security_test.sh:614-616` records an operator decision moving the ~7 min tool-pins self-test out of `lint` to bound its time | decision: move to its own required job with a CFC-6 job-pin like `:617-619`, or record why `lint` takes it in `decisions.md`. Recommended: own job |
| B3 | NOTE | claude/opus:spec | yes | `spec.md:218` (DTP-7) says CWS-6 fires at `ci_workflow_security_test.sh:590`; it is now `:592` after this branch added `:457-458` | fix: drop the line number from the spec |
| B4 | NOTE | claude/opus:spec | yes (known 2.3 gap) | `consistency_toolchain_pins_test.sh:566-579`: any dotted number in a mise.toml or Taskfile.yml comment fails, not only a tool version | accept as stronger-than-spec and state it in DTP-5 |
| B5 | NOTE | claude/opus:spec | yes | `:785-786`: one dead file pattern fails a manager that is otherwise alive | accept as deliberate rot check and state it in DTP-4 |
| B6 | NOTE | claude/opus:spec | yes | the Renovate model at `:824` ignores `extends` presets (`renovate.json:3-6`) | comment the limit in the model (already a recorded G3 gap) |

Not re-found (queued B fixes already): shasum fallback on macOS; helm-docs digest not checked as
64 hex; CFC-2 `latest` via any Taskfile var (`consistency_tool_pins_test.sh:159-163`);
case-sensitive `uses:`; no `task = "latest"` case.

Compliance (claude/opus:spec): DTP-1 holds (stronger), DTP-2 holds, DTP-3 holds, DTP-4 holds
(stronger), DTP-5 holds (stronger), DTP-6 holds, DTP-7 holds. None contradicted or absent.
Unspecified behaviour: `guard_modes` SIGPIPE fix; golangci-lint joins the shared group, so a red
golangci-lint bump can hold Task, gitleaks, git-cliff and govulncheck back; the old guard's
self-wiring check is replaced by CWS-6/CFC-6.
