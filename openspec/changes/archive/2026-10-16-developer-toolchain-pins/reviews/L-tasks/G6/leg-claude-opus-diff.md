## Verdict: CONCERNS

## Findings
- [WARNING] 6.5 closes the `latest` gap for Taskfile vars only. The same gap is still open for Makefile vars — `hack/test/consistency_tool_pins_test.sh:142-143`
  Failure: `CONTROLLER_TOOLS_VERSION ?= latest` (or any Makefile `$(call go-install-tool,…,$(VAR))` var) makes `make_var` return `latest`. The value never becomes `<unresolved:…>`, `resolve` runs `go mod download …@latest`, and CFC-2 passes on a floating pin. That is the exact defect 6.5 fixes three lines lower.
  Fix: apply the `[[ "${ver}" =~ ^v[0-9] ]] || ver="<unresolved:\$(${var})=${ver:-<empty>}>"` check at :143 too, and add a Makefile-var mutant next to the new one at :505-510.
  Confidence: 60 (I did not check whether another gate polices Makefile vars).
- [NOTE] The new comment's premise does not match the code next to it — `hack/test/consistency_tool_pins_test.sh:158,162-163`
  Failure: the comment calls a var holding `latest` "the same floating pin that `@latest` is". But a literal `@latest` is skipped at :158 as "not a pin", so CFC-2 now reds on the indirect form and still passes the direct one. Either the comment is wrong or :158 is a hole.
  Fix: name the gate that rejects a literal `@latest`, or stop skipping it.
  Confidence: 50
- [NOTE] Task 6.1 asked that every sentinel or probe value come from the tree. Some still equal today's pins — `hack/test/consistency_toolchain_pins_test.sh:1988-1990`, `:2050` (govulncheck `v1.9.0`), `:1876` (gitleaks `8.30.1`)
  Failure: after a Renovate bump to v1.9.1, the mutant "govulncheck inline pin returns" writes a stale inline value. It no longer tests the same value written inline, which is the only form today's tree proves. The nested run still passes, so the drift is silent.
  Fix: derive `GV_CUR` from `Taskfile.yml` the way `GC_CUR` is derived (:1324), and use it in both mutants.
  Confidence: 70
- [NOTE] The two installers refuse at different points when no hash tool exists — `hack/lib/verify-sha256.sh:22-29` vs `hack/install-git-cliff.sh:45-52`
  Failure: helm-docs downloads `checksums.txt` and the tarball, then refuses inside `verify_sha256`. git-cliff refuses before any download. The spec ("install nothing") holds either way, but the behaviour differs.
  Fix: optional. Add a pre-flight hash-tool check in `install-helm-docs.sh`, or note the difference.
  Confidence: 80
- [NOTE] The shared `verify_sha256` gained two branches (shasum fallback, no-tool refusal) but its own unit test has no case for either — `hack/lib/verify-sha256.sh:21-29`, `hack/test/verify-sha256_test.sh`
  Failure: every other installer that sources the lib relies on these branches. The only coverage is the helm-docs case inside an 8-minute self-test job, so a regression in the lib is reported far from the lib.
  Fix: add a hidden-PATH case to `verify-sha256_test.sh`.
  Confidence: 60
- [NOTE] The `extends` comment claims more than was checked — `hack/test/consistency_toolchain_pins_test.sh:634-637`
  Failure: it says the 44.115.4 dry-runs "agreed with the model on this config, presets included". Those runs were done in G3, before the `golangci-lint` packageRule (`renovate.json:187-191`) existed.
  Fix: say "on the G3 config", or re-run the dry-run.
  Confidence: 55
- [NOTE] An outside `KOLLECT_TP_NESTED` silently disables the bump control and prints no "skipped" line — `hack/test/consistency_toolchain_pins_test.sh:1265`
  Fix: print `skip: bump control (nested)` when the variable is set.
  Confidence: 40

What I checked and found sound:
- **6.2 CI wiring:** the yq step and the move into `tool-pins-selftest` are correct. The job's depth-1 checkout is fine because the guard only uses `rev-parse` and `ls-files`.
- **6.2 CFC-6:** the loop and the three changed mutants each target their own clause. The BRE `[a-z]*` works because `mutant_rejected` matches with `grep -q`.
- **6.3/6.4 installers:** `hashless_path`'s `^sha[0-9]*sum$` also hides `shasum` itself, so the "nohash" case really has no hash tool. The doubled checksum line is rejected by the anchored regex.
- **6.6:** the only `uses:` reader is `action_version_sites`, and it is now case-insensitive.
- **6.7:** the golangci-lint rule sits after "pinned build tools" and before "go toolchain".
- **6.8:** the DTP-5 stronger-than-spec text matches the tokenizer at :568-579.
- **Commits:** each of the seven commits looks self-contained from its file stat. They were not each run, as the evidence admits.

## Could not check
- I ran no gate, self-test or mutant (read-only leg). The exit codes and times in `evidence/G6.md` are taken as reported, including that the nested bump run passed.
- Whether another gate rejects a literal `@latest` or a Makefile var set to `latest`.
- Real Renovate behaviour of the new golangci-lint group: no dry-run on the final tree.
- The macOS/Darwin path, and the shim branch for hosts with no `shasum`.
- `reviews/B-branch/round-1/verification.md` and `tasks.md` (I went by the task text quoted in the evidence), and the CI runtime of `tool-pins-selftest` on a cold cache.
