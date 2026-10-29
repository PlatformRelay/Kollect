yq is available. Running the meta-test in plain mode (read-only against the repo tree) to verify the evidence claims myself:
## Verdict: CONCERNS

Round-one CRITICAL is verifiably closed (dangling step gone, ci.yaml:446 single; schema ratchet added). Plain-mode meta-test run by me: green, exit 0. Remaining findings are bounded.

**Requirement verdicts (verified against code, not the docs):**
- CWS-1 HOLDS — ci.yaml:175-194; exact invocation pinned (meta-test :174-181); version pin both sides :183-201; mutants for every omission/skip-switch (:637-691).
- CWS-2 HOLDS — `.github/zizmor.yml:14` `rules: {}`, zero suppressions; inline `# zizmor: ignore` rejected (:236-239); artipacked fixed, not suppressed (changelog-sync.yaml:92,136).
- CWS-3 HOLDS — ci.yaml:201-214: PR-only, SHA-pinned action, default threshold, `allow-licenses` only, not in verify-eligibility.sh:19-22; mutants :709-737.
- CWS-4 HOLDS — ci.yaml:55-57, e2e-smoke.yaml:30-38 exact expressions; ref-keyed/unprefixed/unconditional-cancel mutants :741-757.
- CWS-5 HOLDS — ci.yaml:32-42 keeps push:main; workflow-security in verify-eligibility.sh:19-22; failures name verify-eligibility.sh (:315,:320).
- CWS-6 PARTIAL — all step-directly-invoked guards covered (incl. the four round-one orphans, ci.yaml:432-445); wrapper-reached guards contradicted (F2); "any other mode" deliberately narrowed to `--self-test`, documented :347-353.
- CWS-7 HOLDS — 30 mutants + no-op control, message-matched rejections (stronger than spec's "asserts the check fails"); self-test not executed by me (F3).

## Findings
- [WARNING] `schema_steps` never checks composite actions despite its comment, its pass message, and the round-two commit message all claiming "jobs and composite actions alike" — `hack/test/ci_workflow_security_test.sh:520` (else branch is `:`)
  Failure: add a name-only step to `.github/actions/kind-e2e-setup/action.yml` → gate prints "every workflow and composite action declares run or uses" while checking nothing; no CWS-7 mutant covers the action case either.
  Fix: in the non-map branch, assert each `.runs.steps[i]` has run or uses (or correct the three claims to say workflows only).
  Confidence: 90 (verified: `yq eval '.jobs | type'` on both actions returns `!!null`; no mutant exercises the action path)
- [WARNING] CWS-6's "every guard script under hack/test/ that CI runs" is contradicted for ~15 guards whose only CI invocation is inside `hack/docs/verify.sh` via the docs workflow's non-required `task docs:verify` step (e.g. `docs_removed_api_fields`, `security_architecture_docs`, `docs_map_wiring`, `docs_lab_doc_*`, `lab_adr_0707_indexed`) — `hack/docs/verify.sh:15-40`
  Failure: a PR neuters one of those guards; no required check reds (docs "verify" context is absent from verify-eligibility.sh), and the gate cannot even see them — its scanner reads workflow run-body lines only. Round-one flagged this; round two closed only the composite-action half.
  Fix: wire the wrapper-only guards into lint like the four round-one orphans, or amend the spec to define "CI runs" as directly invoked by a workflow step and record the docs guards as out of gate.
  Confidence: 75
- [WARNING] Recorded mutant counts are wrong in three places — recurrence of the round-one evidence-accuracy NOTE — `evidence.md:11` (says 33; actual 30), `evidence.md:58` (stale round-one "19 mutants"), `tasks.md` CWS-7 row ("33 mutants + no-op") and CWS-1 row ("12 mutants"; actual 10)
  Failure: an auditor reconciling evidence against the harness finds three mutually inconsistent numbers for the same harness, undermining the evidence record the change rests on.
  Fix: correct both files to "30 mutants + no-op control" and delete the stale line 58 sentence.
  Confidence: 95 (verified: 30 `*_mutant_rejected` calls; evidence's own CWS-1 bullet lists 10)
- [NOTE] Stronger-than-spec exactness, recorded as deliberate: benign edits (runner label `ubuntu-24.04`, an extra zizmor flag, a second reviewed env var) red the gate until the lock itself is edited — `ci_workflow_security_test.sh:155,160-162,179`
- [NOTE] `reviews/branch/leg-DeepSeek-V4.1-Flash-adversarial.md` is a committed 0-byte file; the register says legs 5/6 but the sixth leg's verdict is recorded nowhere in-repo
- [NOTE] Adjacent out-of-scope observation: docs.yaml:88 keys its concurrency group on `github.ref` for push, so quick main merges can lose the middle Pages run — same rationale CWS-4 exists for, but the spec scopes CWS-4 to ci.yaml/e2e-smoke.yaml only, so no requirement is violated

What the target does that no requirement mentions: the schema ratchet; exact install-step/env/runs-on pins; `deny-licenses`/`warn-only`/job-`continue-on-error` bans; inline zizmor-ignore ban; the four round-one orphan guards promoted into lint (ci.yaml:432-445); zizmor-driven fixes to kind-e2e-setup (env-bound inputs), release.yaml (`cache: false`), changelog-sync (permission scoping, `base64 -w0`); `WORKFLOW_KEY_ALLOWLIST` gains `concurrency` (defended by CWS-4's exact pins).

## Could not check
- `--self-test` (CWS-7) not executed: it mutates tree copies in temp dirs and this session is read-only; the 30 mutants were reviewed by reading only.
- zizmor not installed locally: the "0 findings at high" claim (evidence.md:42-43) rests on the recorded run, not mine.
- GitHub-hosted runtime behaviours (PR-run supersession, dependency-review on real PRs, exact-SHA push runs) — inherently post-merge; tasks.md marks them so.
- Anything outside this repository (workbench records, prior sessions) — out of bounds per instructions.
