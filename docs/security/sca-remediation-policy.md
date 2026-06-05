# SCA remediation policy

Software Composition Analysis (SCA) covers **third-party dependencies** — `go.mod`, GitHub Actions,
and container base images. Application SAST and secret scanning are out of scope here; see
[Coding standards § Security](../development/coding-standards.md#security) and
[SECURITY.md](../../SECURITY.md).

## OSPS-VM-05.01 compliance

> **OSPS-VM-05.01:** While active, the project documentation MUST include a policy that defines a
> threshold for remediation of SCA findings related to vulnerabilities and licenses.

This named policy satisfies [OpenSSF OSPS-VM-05.01](https://baseline.openssf.org/versions/2026-02-19.html)
while kollect is active. It defines **remediation thresholds** for Software Composition Analysis
(SCA) findings on **vulnerabilities and licenses**, and the process to identify, prioritize, and
remediate them (per OSPS recommendation).

| OSPS control | How this policy satisfies it |
| --- | --- |
| **VM-05.01** | Threshold table below — severity bands, calendar-day SLAs, license classes |
| **VM-05.02** | [Pre-release gates](#enforcement-model) — Trivy + SBOM review before `v*.*.*` tags |
| **VM-05.03** | `govulncheck` on every push/PR; violations block merge in practice via CI signal and maintainer policy (see [Enforcement model](#enforcement-model)) |

!!! note "Scope"
    Secret scanning (`gitleaks`) and Go SAST (golangci-lint, CodeQL) run in the same CI pipeline but
    are governed separately.

## Remediation thresholds

**Clock starts** when a finding is first reported by govulncheck, a Dependabot alert, Trivy, or manual
SBOM/license review — whichever occurs first.

### Vulnerability findings

Severity follows the [Go vulnerability database](https://vuln.go.dev/) and GitHub Advisory Database
(CVSS v3.x where published).

| Severity band | CVSS / examples | Remediation threshold | If threshold exceeded |
| --- | --- | --- | --- |
| **Critical** | ≥ 9.0; RCE in reachable operator or UI path | **7 calendar days** | Treat as release blocker; escalate issue |
| **High** | 7.0–8.9; auth bypass, significant exposure | **30 calendar days** | Same |
| **Medium** | 4.0–6.9; limited or hard-to-trigger impact | **90 calendar days** | Track in Dependabot/issue |
| **Low** | &lt; 4.0; defense-in-depth | **Next minor release** | Best-effort advisory |

**Zero-tolerance gates** (no SLA — must be fixed or excepted before merge/release):

| Finding | Merge | Tagged release |
| --- | --- | --- |
| **Reachable vulnerability** (`govulncheck` reports it in `./...`) | Must pass `vulncheck` CI job | Must pass on release commit |
| **Fixable CRITICAL/HIGH** in release OCI images (Trivy, `ignore-unfixed: true`) | N/A (images built at release) | Release workflow fails |

Findings that do **not** appear in govulncheck (e.g. MEDIUM in a base image with no module import)
follow the severity-band SLA and are reviewed before tagging.

### License findings

Kollect is [MIT-licensed](../../LICENSE). Dependencies must remain compatible with distributing the
operator and UI under MIT.

| License class | SPDX examples | Remediation threshold | Merge / release |
| --- | --- | --- | --- |
| **Allow** | MIT, ISC, BSD-2/3-Clause, Apache-2.0, Unicode-3.0, Zlib | None — permitted | Merge allowed |
| **Review** | MPL-2.0, LGPL-2.1, LGPL-3.0 (library use, attribution in SBOM) | **90 calendar days** to confirm compatibility | Merge after review; SBOM spot-check at release |
| **Deny** | GPL-2.0, GPL-3.0, AGPL-3.0; proprietary/custom EULA; `UNKNOWN` / missing | **Before merge** (target **30 calendar days** if found post-merge) | Remove, replace, or [documented exception](#exceptions-and-deferrals) |

Transitive dependencies with **Deny** licenses: upgrade the parent module, replace the chain, or
pursue an exception.

## Identification (detection)

These tools **find** SCA findings; remediation deadlines are in the table above.

| Tool | Finds | When | Workflow |
| --- | --- | --- | --- |
| [**govulncheck**](https://go.dev/security/vuln/) | Known Go CVEs in **imported** packages | Every push and PR | [`ci.yaml` job **vulncheck**](../../.github/workflows/ci.yaml) · `task vulncheck` |
| [**Dependabot**](https://docs.github.com/en/code-security/dependabot) | Advisory DB alerts; update PRs for `go.mod` and Actions | Continuous + weekly | [`.github/dependabot.yml`](../../.github/dependabot.yml) |
| [**Trivy**](https://github.com/aquasecurity/trivy) | Fixable CRITICAL/HIGH in release images | On `v*.*.*` tag | [`release.yaml`](../../.github/workflows/release.yaml) |
| **Release SBOM** | SPDX inventory for license review | On release | `sbom.spdx.json` / `sbom-ui.spdx.json` ([ADR-0705](../adr/0705-release-supply-chain.md)) |
| **`depguard` / `gomodguard`** | Blocklisted imports and modules (`logrus`, `pkg/errors`, …) | Every push and PR | `task lint` in [`ci.yaml`](../../.github/workflows/ci.yaml) |

Contributors run `task vulncheck` locally before opening a PR.

Automated license scanning beyond import blocklists is not yet in CI; release SBOMs and pre-add
review provide audit evidence until then.

## Remediation process

When a finding is open, address in this order:

1. **Upgrade** — bump module or transitive (`go get`, Dependabot security PR).
2. **Replace** — swap to a maintained alternative if upstream has no fix.
3. **Remove** — drop unused dependency (`go mod tidy`; re-run `govulncheck`).
4. **Defer** — documented exception with expiry (see below); does not override Trivy release gates.

**Prioritization** when multiple findings are open:

1. Critical / High with known fixes (govulncheck failures, Dependabot security PRs).
2. Deny-class licenses on direct dependencies.
3. Medium vulnerabilities and Review-class licenses pending compatibility analysis.
4. Low severity and version-update hygiene (Dependabot grouped PRs).

## Enforcement model

Be explicit about what automation **blocks** vs what maintainers **track** under SLA.

| Control | Mechanism | Blocks GitHub merge? | Blocks release? |
| --- | --- | --- | --- |
| **Reachable Go CVEs** | `govulncheck` CI job | No (not a required branch check) — **red job + maintainer policy**; contributors MUST fix before merge | Yes — release only from green CI on tag commit |
| **Blocklisted modules** | `depguard` / `gomodguard` in `task lint` | Same as vulncheck (CI signal) | Same |
| **Dependabot advisories** | GitHub alerts + security PRs | No — **SLA-tracked** (7 / 30 / 90 days by severity) | Reviewed before tag |
| **Image CVEs** | Trivy on `ghcr.io/konih/kollect` (+ UI) | N/A | **Yes** — fixable CRITICAL/HIGH fails release workflow |
| **License (Deny)** | Manual / SBOM review + lint blocklists | Maintainer blocks merge | SBOM spot-check before tag |

GitHub branch protection requires **`preflight`** and **`test`** only
([CONTRIBUTING.md](../../CONTRIBUTING.md)). All other CI jobs (including **vulncheck** and **lint**)
run on every PR; maintainers treat failing SCA jobs as merge blockers even when not branch-protected.

### Pre-release checklist (OSPS-VM-05.02)

Before pushing a `v*.*.*` tag:

- Green **vulncheck** on the release commit.
- **Trivy** — no fixable CRITICAL or HIGH on shipped images.
- **SBOM** — no new Deny-class licenses without a current exception.

See [RELEASE.md](../RELEASE.md).

## Exceptions and deferrals

Findings that cannot be remediated immediately (no upstream fix, false positive, documented
non-reachable path) may be **deferred** only with written approval **before** merge or release.

### Required record

Each deferral MUST include: package/module, advisory or license ID, severity/class, rationale,
compensating controls, **owner**, and **expiry date** (re-review at least every **90 days**).

Record in **one** of:

- GitHub issue labeled `security` (preferred for time-bound CVE deferrals);
- ADR in `docs/adr/` (policy-level or long-lived);
- [SECURITY.md § Exceptions](../../SECURITY.md) (short-lived govulncheck suppressions only).

### Valid deferral reasons

- **No fix available** — upstream advisory lists no patched version; issue tracks upstream.
- **Not reachable** — analysis shows code path never calls affected symbol (prefer upgrade when fix exists).
- **Accepted risk** — pre-alpha operator with compensating control; maintainer sign-off required.

Deferrals do **not** override **Trivy** release gates for fixable CRITICAL/HIGH in shipped images.

## Roles

| Role | Responsibility |
| --- | --- |
| **Contributors** | Run `task vulncheck`; do not introduce Deny-class licenses; open issues for deferrals |
| **Maintainer** | Triage Dependabot alerts within SLA; do not merge red vulncheck/lint; approve deferrals |
| **Release manager** | Verify Trivy + SBOM gates before `v*.*.*` tags |

## Related documents

- [SECURITY.md](../../SECURITY.md) — disclosure, scanning overview, exception stub
- [Coding standards § Security](../development/coding-standards.md#security) — contributor CI gates
- [ADR-0104: Security model](../adr/0104-security-model.md) — runtime threat model
- [ADR-0705: Release supply chain](../adr/0705-release-supply-chain.md) — SBOM, Trivy, Dependabot
- [ADR-0706: Testing merge gates](../adr/0706-testing-merge-gate-architecture.md) — CI job matrix

## Revision history

| Date | Change |
| --- | --- |
| 2026-06-05 | Initial policy (OSPS-VM-05.01) |
| 2026-06-05 | OSPS mapping, unified thresholds, detection vs enforcement split |
