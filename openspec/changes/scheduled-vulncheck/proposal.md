# Proposal

## Why

`govulncheck` runs only in the `vulncheck` job of `.github/workflows/ci.yaml`, on pull requests and pushes
that change code (`needs: [changes]`, `if: needs.changes.outputs.code == 'true'`). The vulnerability
database changes without any commit to this repository, so a new advisory against a module or the
standard library that kollect already calls turns `vulncheck` red on the next code-affecting run of every
branch: each new PR, and each open PR as soon as it is rebased or re-run. `vulncheck` is a required check, so the queue stops until someone fixes the
advisory. Nothing reports it before that.

This has happened twice:

- **2026-10-25:** GO-2026-6809, a stdlib advisory. The Go 1.27.3 bump came from an external contributor's
  PR.
- **2026-11-12/13:** three advisories at once. They turned `vulncheck` red on 9 of 10 open Renovate PRs:
  - GO-2026-6859 (net/http, fixed in go1.27.4);
  - GO-2026-6888 (`github.com/andybalholm/brotli`, indirect, fixed in v1.2.7);
  - GO-2026-6865 (`github.com/klauspost/compress`, indirect, fixed in v1.20.2).

  No single Renovate PR could pass. They were fixed together in #548.

The other signals do not cover this:

- **Dependabot:** raised no alert for any of the three 2026-11 advisories. The repository's alerts
  list holds nothing for brotli, compress or the stdlib (`gh api repos/PlatformRelay/Kollect/dependabot/alerts`,
  checked 2026-11-16).
- **Renovate:** does not update `// indirect` gomod dependencies by default, and its
  `osvVulnerabilityAlerts` covers direct dependencies only. A rule enabling indirect updates would have
  bumped brotli eventually, on Renovate's schedule, but it would not have said that an advisory was
  waiting (docs.renovatebot.com, gomod manager and
  configuration options; read 2026-11-16).
- **`Go patch lag`:** weekly (`23 6 * * 1`), and it covers only the toolchain. Its last green run on
  2026-11-09 came before go1.27.4.

Only `govulncheck` itself sees these advisories, with its reachability analysis, so it has to run without
a commit.

## What Changes

- A new workflow, `.github/workflows/vulncheck-scheduled.yaml`, named `Vulnerability scan`:
  - runs `task vulncheck` on the default branch once a day and on `workflow_dispatch`, through a small
    script, `hack/ci/vulncheck-scheduled.sh`. The script keeps govulncheck's output and status, and adds
    one `FAIL: <GO id> <title>` line per advisory. The `ci-failure` issue quotes only lines containing
    `FAIL`, so this is what puts the advisory into the issue. A failure without an advisory gets one
    "tool, network or build failure" line instead;
  - has `permissions: contents: read`, no secrets, and no `if:` or `continue-on-error` that could soften
    the step.
- `Vulnerability scan` is added to the `on.workflow_run.workflows` list of
  `.github/workflows/ci-failure-report.yaml`. A failing scheduled scan then opens, or comments on, one
  `ci-failure` issue (CFR-1 to CFR-3, unchanged).
- A guard test, `hack/test/vulncheck_schedule_test.sh`, pins the workflow's shape. It has a `--self-test`
  that proves each check can fail, and the `lint` job runs it in both modes, next to the go-patch-lag
  guard. `hack/test/ci_failure_report_test.sh` expects the extended list and gains one self-test case for
  losing the new entry.
- Docs:
  - `docs/security/sca-remediation-policy.md` (VM-05.03 and the tooling table) and
    `docs/development/testing.md` (scheduled tiers) say govulncheck also runs daily on `main`;
  - the SLA clock text says a vulnerability first named by a scheduled-scan issue starts its clock there.
    A "no advisory" issue is a tool failure and starts no clock.

## Out of scope

- Update automation: Renovate `vulnerabilityAlerts`, `osvVulnerabilityAlerts`, or enabling indirect
  gomod updates. This change is about detection: an advisory must be visible on the day it lands, whatever
  supplies the fix. Enabling indirect updates is a separate Renovate policy question (more PRs, each needing
  review) and can be raised on its own.
- Auto-fixing: the scan reports and never opens a PR.
- Scanning the shipped image or binary (`govulncheck -mode binary`, Trivy on a schedule).

## Impact

- CI only: one new scheduled workflow, about two minutes of runner time a day; no operator code, chart or
  CRD change.
- A red scan does not block pull requests. It opens a `ci-failure` issue, and that issue stays open, with
  one comment per failing day, until a scan passes.
- The scan fails closed. A vuln.go.dev or module-proxy outage opens an issue as well. Its FAIL line says
  "no advisory", and the next green day closes it (CFR-1.6).
