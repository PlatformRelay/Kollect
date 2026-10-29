# Evidence — ci-workflow-hardening (implementation)

Recorded 2026-10-05 in the disposable worktree that produced the branch; every command below
was run against the branch tip, not against main.

## hack/test/ci_workflow_security_test.sh (CWS-1..CWS-7 meta-test)

Plain mode against the branch tree: all CWS-1..CWS-6 assertions green
(`All workflow-security meta-tests passed.`).

`--self-test` (CWS-7): no-op control green, 34 mutants each rejected with the exact message of
the assertion the mutation was built to trip:

- CWS-1: audit without `--offline`; loosened `--min-severity`; renamed config; scope narrowed
  to `workflows/`; job with `if` (skip switch); job deleted; install step without the version
  pin; install step swapped for `pipx`; audit step with a `shell:` override; install step with
  a second env binding (KOLLECT_FORCE_SHA256) — all rejected with the pinned-invocation /
  skip-switch / version-pin messages.
- CWS-2: bare suppression appended under `rules:`; inline `# zizmor: ignore[...]` in a
  workflow — each rejected naming the missing comment or the unreviewed inline suppression.
- CWS-3: `if` dropped (not PR-only), `fail-on-severity: critical` without `# why:`,
  `deny-licenses` used, `warn-only` used, job-level `continue-on-error`,
  `dependency-review` added to release eligibility → each rejected.
- CWS-4: group keyed by `github.ref`; group without the `github.workflow` prefix;
  `cancel-in-progress: true` → each rejected with the exact-expression message.
- CWS-5: `push` trigger deleted; push branch moved off `main`; `workflow-security` dropped
  from `verify-eligibility.sh` → each rejected.
- CWS-6: guard run only from a non-required job; a `--self-test` mode parsed but never
  invoked; this meta-test's own lint wiring removed; a guard invoked from a composite action
  → each rejected with the message naming the script and mode.
- Schema ratchet: a dangling name-only step (run and uses both absent — GitHub rejects the
  whole workflow at load) → rejected. Added after the round-one review found exactly this
  defect in ci.yaml.

Pre-wiring red (task 1.1's "watch it fail on the missing wiring"): before the lint wiring
existed, the gate reds with `FAIL: CWS-6: this meta-test (hack/test/ci_workflow_security_test.sh)
is not itself run by ci.yaml in its 'bare' mode -- wire it into the lint job next to
ci_docs_gate_test.sh, plain and --self-test as separate steps`.

## zizmor audit (zizmor 1.30.1, installed by hack/install-zizmor.sh)

- `zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/`
  on the branch tip: exit 0, `No findings to report. Good job! (36 ignored, 27 suppressed)`.
- At `--min-severity=medium`: 0 findings (the single medium on the pre-fix tree was
  `artipacked` on changelog-sync.yaml — fixed, see below).
- At `--min-severity=low`: 24 findings, all `self-repository`, all Regular persona — style
  hygiene below the gate threshold (9 in ci.yaml, 15 across the other workflows/actions);
  each is mechanically fixable if the threshold ever drops.
- 27 "suppressed" are zizmor's default Pedantic-persona filtering, NOT config suppressions
  (`.github/zizmor.yml` has `rules: {}`).
- The pinned digests were re-checked against the GitHub release API's asset `digest` fields
  (`gh api repos/zizmorcore/zizmor/releases/tags/v1.30.1`): the darwin/arm64 pin matches
  exactly, and the linux/x86_64 pin is verified by every CI run.
- The `artipacked` medium finding on changelog-sync.yaml was fixed, not suppressed:
  `persist-credentials: false` on the checkout + per-push `GIT_CONFIG_*` extraheader with the
  app token (the actions/checkout pattern; `base64 -w0` so the header cannot wrap), keeping
  the exact `git push origin HEAD:main` spelling the release guard test recognises.

## Gate runs on the tree

- `bash hack/test/ci_workflow_security_test.sh` — green (plain mode).
- `bash hack/test/ci_workflow_security_test.sh --self-test` — green (34 mutants + no-op).
- `bash hack/test/dist_ci_wiring_test.sh` — green after `WORKFLOW_KEY_ALLOWLIST` gained
  `concurrency` with the CWS-4 justification.
- `bash hack/test/ci_docs_gate_test.sh` — green (runs slow locally with signed global git
  config; neutralised for the local run).
- `bash hack/test/changelog_sync_release_guard_test.sh` — green (push-spelling matcher accepts
  the `GIT_CONFIG_*` push shape; PyYAML + GPG-free git env required locally — CI installs
  `python3-yaml` and signs nothing).
- `bash hack/test/sonar_ko_08_workflow_permissions_test.sh` — green (the changelog-sync
  `permission-contents` addition does not move the scoped permissions).
- `finalizer_cleanup_e2e_test.sh`, `e2e_mt_repro_harden_test.sh`,
  `e2e_webhook_existing_cluster_test.sh`, `demo_03_hero_smoke_test.sh` — green; these four
  guards were CWS-6 orphans (invoked only from non-required jobs: the nightly and extended
  e2e suites and a smoke-only job) and are now also run as required steps of `lint`.

## Known environment caveat for local runs

`changelog_sync_release_guard_test.sh` and `ci_docs_gate_test.sh` need PyYAML (CI installs
`python3-yaml`) and a GPG-free git environment; locally they run green with
`PATH=<venv>:$PATH GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null`.
