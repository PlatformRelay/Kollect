## Verdict: CONCERNS

All seven requirements are met in the code. I found nothing that contradicts the spec. The concerns are places where the guard checks more than the spec asks for, one stale line reference in the spec, and the lint job's run time. The cited lines are at HEAD; `consistency_toolchain_pins_test.sh` is shortened to `ctp.sh` below.

| Req | Status | Where it is decided |
|---|---|---|
| DTP-1 | holds, and stronger than spec | `ctp.sh:1050-1061`, floor at `:343`. It scans every YAML under `.github/`, requires `ci.yaml` and `kind-e2e-setup` to have a site, and fails a setup-task step that has no `version:` (`:200`). |
| DTP-2 | holds | `ctp.sh:967-1011`. No floor is set, as the spec says. `consistency_tool_pins_test.sh:470,481` now mutates the variable instead of the inline pin. |
| DTP-3 | holds | gitleaks `:1019-1026`, git-cliff `:1028-1035`, shellcheck with `first3` `:1037-1047`, digest check across every OS and architecture pair `:352-371`. |
| DTP-4 | holds, and stronger than spec | Python model `:643-848`, coverage and grouping `:895-946`. It also fails when any single file pattern matches nothing (`:785`), even if the manager is alive through another pattern. |
| DTP-5 | holds, and stronger than spec | `check_mise` `:491-516`, `comment_findings` `:527-608`. Any dotted number in a comment that isn't next to its pin fails, not only tool versions. |
| DTP-6 | holds | `hack/install-git-cliff.sh:43-56` (`sha512`), `hack/install-helm-docs.sh:48-55`, `UNVERIFIED_BY_DESIGN=()` at `ci_fetch_lib_hardening_test.sh:379`, empty limit `ctp.sh:102`, valid/tampered/missing/other-asset runs `:1526-1581`. |
| DTP-7 | holds | Two separate steps in `lint` (`ci.yaml:567-570`). CFC-6 picks the guard up through the `consistency_*_test.sh` glob (`ci_workflow_security_test.sh:602`). Mutants exist for every requirement, and every case from the old mise guard is ported (`:1587-1657`). |

## Findings
- [WARNING] The new `--self-test` adds about 4 minutes to the `lint` job, against a same-day rule that keeps slow self-tests out of it — `.github/workflows/ci.yaml:570`
  Failure: G4 records 233s for this self-test (`evidence/G4.md:161`). `ci_workflow_security_test.sh:614-616` says the ~7 min tool-pins self-test was moved to its own job so lint's run time stays bounded (operator decision 2026-10-16). This change puts a self-test of similar weight back into lint.
  Fix: run it in `tool-pins-selftest`, or in a new required job, and add a CFC-6 job-pin like `:617-619`. Or record in decisions.md why lint is acceptable here.
  Confidence: 70
- [NOTE] The spec's line reference for CWS-6 is out of date because of this branch's own edit — `hack/test/ci_workflow_security_test.sh:592`
  Failure: DTP-7 says CWS-6 fires "at `ci_workflow_security_test.sh:590`". The two comment lines added at `:457-458` moved it to `:592`.
  Fix: change the spec to `:592`, or drop the line number.
  Confidence: 95
- [NOTE] DTP-5 fails on any dotted number in a comment, which is broader than "tool-version restatement" — `hack/test/consistency_toolchain_pins_test.sh:566-579`
  Failure: a Taskfile.yml comment such as `# see spec section 2.3` turns the guard red as "restates a version away from its pin", although it names no tool.
  Fix: accept it and write it down as stronger than spec, or only flag a number that follows a tool name.
  Confidence: 75
- [NOTE] DTP-4 fails a manager with one dead file pattern even when its other patterns work — `ctp.sh:785-786`
  Failure: a manager listing an optional or future file is a legitimate config, and it fails although the spec only requires that the manager match at least one tracked file.
  Fix: accept it as a deliberate rot check and say so in the spec; no code change needed.
  Confidence: 80
- [NOTE] The Renovate model ignores the `extends` presets, so its group check can't see rules that come from them — `ctp.sh:824`, `renovate.json:3-6`
  Failure: if a `config:recommended` preset ever set a `groupName` for one of these depNames, the guard would report the wrong group. Only the one-off Renovate dry-run in G3 covers this.
  Fix: comment that the model leaves presets out, or fail closed on an `extends` that isn't on an allow-list.
  Confidence: 40

**Things the change does that no requirement mentions:**
- `guard_modes` replaces `grep -q` to avoid a random SIGPIPE under pipefail (`ci_workflow_security_test.sh:457-459`).
- A new golangci-lint Renovate manager puts golangci-lint bumps into the shared "pinned build tools" group (`renovate.json`, golangci-lint manager). A red golangci-lint bump can now hold back Task, gitleaks, git-cliff and govulncheck, and the spec only isolates shellcheck.
- The old guard's check that it was wired into CI is gone; CWS-6/CFC-6 now cover that.
- Edits to docs, comments and `.gitignore`.

## Could not check
- I did not run any guard or self-test, by brief. Pass results and timings come from `evidence/G4.md` only.
- I did not check upstream checksum formats (git-cliff `.sha512`, helm-docs `checksums.txt`) or the Renovate 44.115.4 dry-run in `evidence/G3.md` against real Renovate behaviour.
- I did not check whether `lint` is in the repository's branch-protection required set; I only saw the code's `elig_names`.
