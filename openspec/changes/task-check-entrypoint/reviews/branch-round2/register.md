## Unified verdict: BLOCK   (legs ok: 4/4)

| # | Sev | Claim (one line) | Where | Models | Legs | Conf |
|---|---|---|---|---|---|---|
| 1 | CRITICAL | `task check` skips `task_check_test.sh` itself with a false "declared in its header" reason — the sweep greps whole guard files for `requires docker`, matching the guard's own assertion/mutant literals; the same unregistered content-grep also decides `--self-test` mode, so any future guard merely mentioning the phrase silently drops out | hack/check.sh:73,82; task_check_test.sh:121,313 | DeepSeek,Qwen,GLM (3) | adv,sec,spec,fit | 100 |
| 2 | CRITICAL | `run_gate go-mod` declared twice (both added by the fix commit): gate runs twice per `task check`, line 62 is a weaker `go.sum`-only duplicate pinned by nothing; no uniqueness assertion exists | hack/check.sh:50,62 | DeepSeek,Qwen,GLM (3) | adv,sec,spec,fit | 100 |
| 3 | CRITICAL | `format:check` dropped from `GATE_COMMAND` pin — a required ci.yaml `lint` step can vanish from check.sh with the guard staying green (round-one coverage shrank) | task_check_test.sh:71-86 vs check.sh:37, ci.yaml:284 | DeepSeek,GLM (2) | adv,spec | 100 |
| 4 | CRITICAL | CWS-2 suppression gate bypassed by flow-inline `ignore:` and quoted keys (shipped awk executed: rc=0); round-one fix added a mutant only for the spelling it already catches and overclaims closure in its comment | ci_workflow_security_test.sh:253-289,835-841,234-238 | Qwen (1) | sec | 90 |
| 5 | WARNING | Change record inaccurate: evidence.md duplicates the exclusion paragraph verbatim; tasks.md claims "4 mutants" where 7 exist; stale "only ci_workflow_security parses --self-test" | evidence.md:14-18; tasks.md:5-7,34 | DeepSeek,GLM (2) | adv,spec,fit | 100 |
| 6 | WARNING | Tool prereqs undeclared: yq absent from mise.toml/docs (CI installs ad hoc), demo_04 guard hard-fails without kubectl/kustomize — reds on exactly the no-Docker machine TCE-1 promises | mise.toml:39; demo_04_samples_kustomize_test.sh:21-28 | DeepSeek,GLM (2) | adv,fit | 95 |
| 7 | WARNING | An exclusion whose reason token is deleted entirely passes the guard — prefix-strip no-ops, reason falls back to whole line (verified by execution: MISSED) | task_check_test.sh:176 | GLM (1) | fit | 90 |
| 8 | WARNING | TCE-3 mutant gap: no mutant inserts a name into `required_checks` (TCE-4 promises mutants for TCE-1..TCE-3) | task_check_test.sh:202-223,332-337 | DeepSeek (1) | adv | 80 |

## Disagreements
- Verdict split 3×BLOCK vs GLM-fit CONCERNS: GLM-fit verified round-one findings genuinely closed and graded fitness; it saw the too-broad docker grep (70) but not that it skips the meta-test itself.
- Spec leg ruled TCE-3/TCE-4 "holds"; DeepSeek found TCE-3 mutant coverage incomplete and GLM-fit proved an existing mutant misses the deleted-reason shape — the green self-test overstates coverage.
- format:check drop: DeepSeek graded WARNING (required CI step unpinned) vs GLM-spec NOTE ("guard-strength shrinkage, not a spec violation").
- Qwen's CWS-2 CRITICAL is the only leg that ran the shipped awk; DeepSeek and GLM-fit both left CWS-2 explicitly unverified — single-source CRITICAL.

## Nobody could check
- Full `task check` end-to-end — no leg (or author) ever ran it (writes bin/, coverage.out, mktemp trees; downloads pinned tools/envtest/helm); the ~73-guard sweep's combined runtime/tool deps unmeasured; nothing in CI executes check.sh itself.
- Either guard's `--self-test` not reproduced (mutates mktemp copies; the CWS self-test timed out at 120 s mid-mutants) — green self-test claims are attested, not reproduced.
- zizmor upstream semantics (rule-level `disable:` in flow/quoted form) and the round-one quoted-key fix — no zizmor binary available; Qwen's parser acceptance is inference from standard YAML.
- Live branch-protection list — whether `dependency-review` (and the other excluded checks) are actually required; exclusion reasons read from verify-eligibility.sh:19-22, not the real setting.
- Whether the ~70 other swept guards are genuinely Docker/network/tool-free, or need Docker without declaring it — would require running the sweep.
