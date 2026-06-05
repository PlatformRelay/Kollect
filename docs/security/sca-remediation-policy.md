# SCA remediation policy

> Thresholds and process for remediating Software Composition Analysis (SCA) findings in
> **kollect** — dependency vulnerabilities and license compatibility. Satisfies
> [OpenSSF OSPS-VM-05.01](https://baseline.openssf.org/) (documented remediation thresholds for
> SCA findings).

!!! note "Scope"
    This policy covers **third-party dependencies** (`go.mod`, GitHub Actions, container base
    images). Application-level static analysis (SAST) is governed separately in
    [Coding standards § Security](../development/coding-standards.md#security). Secret scanning
    (`gitleaks`) is out of scope for SCA but runs in the same CI pipeline.

## Policy summary

| Finding type | Threshold | Merge to `main` | Tagged release |
| --- | --- | --- | --- |
| **Reachable vulnerability** (govulncheck) | Zero tolerance | Blocked — CI **vulncheck** job fails | Must pass before tag |
| **Fixable CRITICAL/HIGH** (image scan) | Zero tolerance | N/A (images built at release) | Blocked — Trivy fails release workflow |
| **MEDIUM/LOW** (no govulncheck hit) | Remediate per SLA below | Advisory only | Review before release |
| **Denied license** on direct dependency | Zero tolerance | Blocked — revert or replace before merge | Must pass SBOM review |
| **Copyleft / unknown license** | Exception required | Blocked until documented exception | Exception must be current |

## Detection tooling

Findings originate from automated scans and dependency update tooling:

| Tool | What it checks | When | CI / workflow |
| --- | --- | --- | --- |
| [**govulncheck**](https://go.dev/security/vuln/) | Known Go CVEs in **imported** packages | Every push and PR | `task vulncheck` — job **vulncheck** in [`.github/workflows/ci.yaml`](../../.github/workflows/ci.yaml) |
| [**Dependabot**](https://docs.github.com/en/code-security/dependabot) | Advisory DB alerts; version-update PRs for `go.mod` and Actions | Continuous + weekly PRs | [`.github/dependabot.yml`](../../.github/dependabot.yml); repo-level security alerts |
| [**Trivy**](https://github.com/aquasecurity/trivy) | Fixable CRITICAL/HIGH in release OCI images | On `v*.*.*` tag | [`.github/workflows/release.yaml`](../../.github/workflows/release.yaml) (`ignore-unfixed: true`) |
| **Release SBOM** | SPDX inventory for license review | On release | `sbom.spdx.json` / `sbom-ui.spdx.json` ([ADR-0705](../adr/0705-release-supply-chain.md)) |
| **CodeQL** | Go SAST (not SCA) | Push/PR + weekly | [`.github/workflows/codeql.yaml`](../../.github/workflows/codeql.yaml) |

Contributors run `task vulncheck` locally before opening a PR ([Coding standards](../development/coding-standards.md#security-and-supply-chain)).

## Vulnerability severity and remediation windows

Severity follows the [Go vulnerability database](https://vuln.go.dev/) and GitHub Advisory
Database ratings (CVSS v3.x where published). **Clock starts** when a finding is reported by
govulncheck, a Dependabot alert, or Trivy — whichever occurs first.

| Severity | Examples | Remediation target | Merge / release gate |
| --- | --- | --- | --- |
| **Critical** | CVSS ≥ 9.0; RCE in reachable operator or UI code path | **7 calendar days** | CI blocks merge if govulncheck reports it; release blocked if image scan finds fixable CRITICAL |
| **High** | CVSS 7.0–8.9; auth bypass, significant data exposure | **30 calendar days** | Same — govulncheck blocks merge; Trivy blocks release for fixable HIGH |
| **Medium** | CVSS 4.0–6.9; limited impact or hard-to-trigger | **90 calendar days** | Does not block merge unless govulncheck flags it (then treat as in-scope) |
| **Low** | CVSS &lt; 4.0; defense-in-depth | **Next minor release** or best effort | Advisory; track via Dependabot/issue |

### Pre-merge enforcement

The **vulncheck** CI job runs `govulncheck` over `./...` and **must pass** before merge
([ADR-0706](../adr/0706-testing-merge-gate-architecture.md)). This enforces a **zero known
reachable vulnerability** threshold on every change to `main`.

Remediation steps (in order):

1. **Upgrade** — bump the affected module or transitive dependency (`go get -u`, Dependabot PR).
2. **Replace** — swap to an maintained alternative if upstream has no fix.
3. **Remove** — drop the dependency if unused (`go mod tidy`, verify with `govulncheck`).
4. **Exception** — documented accepted risk (see [Exceptions](#exceptions-and-accepted-risk) below).

### Pre-release enforcement

Tagged releases additionally require:

- Green **vulncheck** on the release commit (same as merge gate).
- **Trivy** scan of `ghcr.io/konih/kollect` and `ghcr.io/konih/kollect-ui` with **no fixable
  CRITICAL or HIGH** findings (`ignore-unfixed: true`).
- Review of release **SBOM** for license compliance (see below).

## License categories

Kollect is [MIT-licensed](../../LICENSE). Direct and runtime dependencies must remain compatible
with distributing the operator and UI under MIT.

### Allowed (no exception)

Permissive licenses that permit proprietary use and modification:

- MIT, ISC, BSD-2-Clause, BSD-3-Clause, Apache-2.0, Unicode-3.0, Zlib

### Allowed with review

Weak copyleft or licenses with notice/patent clauses — permitted when used as **library**
dependencies (not combined into a single GPL work) and attribution is preserved in SBOM/release
notes:

- MPL-2.0, LGPL-2.1, LGPL-3.0 (file-level copyleft)

### Denied (must not merge without exception)

| Category | Examples | Action |
| --- | --- | --- |
| **Strong copyleft** | GPL-2.0, GPL-3.0, AGPL-3.0 | Remove or replace dependency before merge |
| **Proprietary / custom** | Unknown commercial EULA, “all rights reserved” | Do not add |
| **Unknown / missing** | `UNKNOWN`, empty `LICENSE`, unreadable SPDX ID | Identify license or remove; do not merge |

Indirect (transitive) dependencies with denied licenses follow the same rule: upgrade the
parent module, replace the chain, or pursue an exception.

### License remediation process

1. **Before adding a dependency** — confirm license (module repo, `go mod` proxy metadata, or
   [pkg.go.dev](https://pkg.go.dev/)).
2. **On SBOM or manual review finding** — open a tracking issue; target remediation within
   **30 calendar days** for denied licenses, **90 calendar days** for review-category licenses
   that fail compatibility analysis.
3. **At release** — maintainer spot-checks SPDX SBOM assets for new denied licenses.

Automated license scanning in CI is planned; until then, `gomodguard` / `depguard` block known-bad
imports and release SBOMs provide audit evidence ([ADR-0705](../adr/0705-release-supply-chain.md)).

## Prioritization

When multiple findings are open, address in this order:

1. Critical / High vulnerabilities with known fixes (govulncheck failures, Dependabot security PRs).
2. Denied-license direct dependencies.
3. Medium vulnerabilities and weak-copyleft review items.
4. Low severity and version-update hygiene (Dependabot grouped PRs).

## Exceptions and accepted risk

Some findings cannot be remediated immediately (no upstream fix, false positive, unreachable code
path in a transitive module). Exceptions **must** be documented **before** merge or release and
re-reviewed at least every **90 days**.

### Required documentation

Record each exception in **one** of:

- A **GitHub issue** labeled `security` with: module/package, advisory ID (e.g. `GO-2024-…` or
  GHSA), severity, rationale, compensating controls, and **review-by date**; or
- An **ADR** in `docs/adr/` for policy-level or long-lived exceptions; or
- The **Exceptions** subsection of [SECURITY.md](../../SECURITY.md) for short-lived
  govulncheck suppressions.

### Valid exception reasons

- **No fix available** — upstream advisory states no patched version; issue tracks upstream release.
- **Not reachable** — govulncheck reports a symbol your code path never calls; document analysis
  (prefer upgrading anyway when a fix exists).
- **Accepted risk** — pre-alpha operator with documented compensating control (network policy,
  feature disabled); requires maintainer sign-off and review date.

Exceptions do **not** override **Trivy release gates** for fixable CRITICAL/HIGH in shipped images.

## Roles

| Role | Responsibility |
| --- | --- |
| **Contributors** | Run `task vulncheck`; do not introduce denied licenses; open issues for exceptions |
| **Maintainer** | Triage Dependabot/Code Scanning alerts; merge security PRs within SLA; approve exceptions |
| **Release manager** | Verify Trivy + SBOM gates before pushing `v*.*.*` tags ([RELEASE.md](../RELEASE.md)) |

## Related documents

- [SECURITY.md](../../SECURITY.md) — vulnerability disclosure and scanning overview
- [Coding standards § Security](../development/coding-standards.md#security) — contributor CI gates
- [ADR-0104: Security model](../adr/0104-security-model.md) — runtime threat model
- [ADR-0705: Release supply chain](../adr/0705-release-supply-chain.md) — SBOM, Trivy, Dependabot
- [ADR-0706: Testing merge gates](../adr/0706-testing-merge-gate-architecture.md) — CI job matrix

## Revision history

| Date | Change |
| --- | --- |
| 2026-06-05 | Initial policy (OSPS-VM-05.01) |
