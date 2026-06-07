# ADR-0303: Helm / Argo release inventory sample and redaction

> Argo CD `Application` is the primary Helm sample; never export secrets — redact sensitive keys.

**Theme:** 03 · Collection & extraction · **Status:** Current
from Flux `HelmRelease` to Argo `Application`.

## Context

Product requirements call for tested sample CRs including **Helm release metadata**
([REQUIREMENTS.md](../REQUIREMENTS.md)). The first walkthrough uses `apps/v1 Deployment`
([deployment-inventory example](../examples/deployment-inventory.md)).

Helm stores release state in opaque `helm.sh/v1` Secrets (`data.release` is base64+gzip JSON).
Those objects—and GitOps `HelmRelease` `spec.values`—can contain passwords, tokens, and
`secretKeyRef` blocks. Exporting them to Git, Postgres, or the inventory HTTP API without policy
would leak credentials into logs and public demo repos.

User requirement : inventory must expose **chart `version` and `appVersion`** by default;
**deployed values** are useful for some teams but must be optional and gated.

## Decision

### Primary GVK (default demo and first sample) — amended ADR-0201

**`argoproj.io/v1alpha1` / `Application`** (Argo CD)

- Structured `status` sync history and `spec.source` chart metadata — common in Argo CD estates.
- Works with existing JSONPath/CEL extraction.
- `KollectTarget` scopes Applications via namespace/label selectors.

**Amended :** Argo primary per [ADR-0201](0201-crd-model.md). Flux
`HelmRelease` sample may remain secondary.

**Version fields (Argo — lock in contract test):**

| Attribute | Path (candidate) | Notes |
| --- | --- | --- |
| `chartVersion` | `$.status.sync.revision` or history entry chart revision | **Contract test required** |
| `appVersion` | From helm parameters / `status.history` | Validate against golden fixture |
| `revision` | `$.status.history[*].revision` | Ordering validated in CI |
| `valuesChecksum` | `$.status.sync.comparedTo` digest or equivalent | Drift without full values export |

**Contract test (CI):** golden **`Application`** fixture in `test/schema/` or envtest asserts

### Secondary GVK (Flux)

**`helm.toolkit.fluxcd.io/v2` / `HelmRelease`** — optional sample; paths below for Flux shops:

| Attribute | Path | Notes |
| --- | --- | --- |
| `chartVersion` | `$.status.lastAttemptedRevision` | Authoritative for Flux |
| `appVersion` | `$.status.history[0].appVersion` | Contract test for ordering |
| `valuesChecksum` | `$.status.lastAttemptedConfigDigest` | |

**Contract test (CI):** golden `HelmRelease` fixture (secondary) asserts
`history[0]` is newest (compare `lastDeployed` / `version` ordering) and that
`lastAttemptedRevision` matches expected chart version string.

### Secondary GVK (plain Helm; deferred implementation)

**`helm.sh/v1` / `Secret`** (`type: helm.sh/release.v1`, label `owner=helm`)

- Covers `helm install` without Flux.
- **Not** in the public demo sample until Kollect adds a **`helm:` decode path** (or CEL helper)
  that expands `data.release` and exposes `chart.metadata.version`, `chart.metadata.appVersion`,
  and `config` (values).
- **Never** export raw `data.release` or rendered `manifest` blobs.

### Two-tier profile model

| Profile | Audience | Attributes | Values |
| --- | --- | --- | --- |
| **`helm-release-summary`** (default) | All teams, public demo | `releaseName`, `chartVersion`, `appVersion`, `revision`, `status`, `lastDeployed`, chart ref, `valuesChecksum` | No |
| **`helm-release-values-redacted`** (opt-in) | Platform / audit (gated) | Above + scrubbed `spec.values` | Yes, redacted |

Public `config/samples/` and demo Git repos ship **summary only**.

### Redaction policy (values profile and future operator scrub)

**Never extract raw `Secret.data` values by default.** Summary attributes come from Helm release
labels/metadata or Flux `HelmRelease` status—not from decoded `values.yaml` or opaque storage blobs.

Profile attributes use **`optional: true`** and explicit **`type: string`** (or `int` for revision)
so missing history or digest fields do not fail collection on partially reconciled releases.

**Always deny export of:**

- Raw Helm storage: `data.release`, full rendered `manifest`
- Credential carriers: `valueFrom.secretKeyRef`, `envFrom.secretRef`, TLS key material
- Subtrees whose keys match (case-insensitive): `password`, `passwd`, `secret`, `token`,
  `apikey`, `api_key`, `privatekey`, `private_key`, `credential`, `auth`, `clientSecret`,
  `connectionString`

When values extraction is added later, apply the same key filter to **any nested** values map
(JSONPath/CEL scrub before export—not at admission time).

**Replace denied values with:** `{"redacted": true, "reason": "sensitive-key"}` (no hashing or
truncation of secrets).

**Prefer checksum over full values** when drift detection suffices: Flux
`status.lastAttemptedConfigDigest` / `status.history[].configDigest`.

**Governance:** `helm-release-values-redacted` is restricted via **`KollectScope`** and
must use private sinks—not the public demo Git repo.

### Admission webhook (Secret.data guard)

The Profile validating webhook rejects attribute paths that read **`Secret.data`** unless the
Profile carries an explicit opt-in annotation:

```yaml
metadata:
  annotations:
    kollect.dev/allow-secret-extraction: "true"
```

This blocks accidental inventory of base64 credential blobs (including Helm storage
`data.release`) on the default demo path. The public **`helm-release-summary`** sample targets
Flux `HelmRelease` and never references `Secret.data`. Plain-Helm `Secret` profiles remain
deferred until a gated `helm:` decode path exists.

Implementation: `internal/validation/profile.go` (`ValidateProfile`); wired from
`internal/webhook/v1alpha1/kollectprofile_webhook.go`.

### Sample manifests

| File | Purpose |
| --- | --- |
| `config/samples/kollect_v1alpha1_kollectprofile_argo-application-summary.yaml` | Summary-tier Profile (**Argo `Application` GVK — primary**) |
| `config/samples/kollect_v1alpha1_kollecttarget_argo-applications.yaml` | Example Target scoping team Argo Applications |
| `config/samples/kollect_v1alpha1_kollectprofile_helm-release-summary.yaml` | Summary-tier Profile (Flux `HelmRelease` — secondary) |
| `config/samples/kollect_v1alpha1_kollectprofile_helm-release-values-redacted.yaml` | Values-tier Profile (Flux `HelmRelease` — gated, scrubbed `spec.values`) |
| `config/samples/kollect_v1alpha1_kollecttarget_helm-releases.yaml` | Example Target scoping Flux HelmReleases |

Walkthrough: [docs/examples/helm-release-inventory.md](../examples/helm-release-inventory.md) (Argo
`Application` primary; Flux `HelmRelease` secondary section).

### Implementation phases

1. **Now:** Argo **`Application`** **contract test first** (`internal/collect/argo_application_contract_test.go`),
   then summary profile + target samples; Flux `HelmRelease` sample remains secondary; Profile webhook
   blocks `Secret.data` without opt-in; webhook **requires `cel:` prefix** on CEL expressions
   ([ADR-0302](0302-cel-jsonpath-extraction.md)); JSONPath **filter** validation **warn-only** in Phase 1.
2. **Phase 2+:** **`helm:release.<field>`** attribute prefix — decoder expands `data.release` gzip JSON
   (`chartVersion`, `appVersion`, `config` values); never export raw blob.
3. **Phase 2:** export-time scrub via global operator **`scrubKeys[]`** denylist; per-attribute
   `redact: true` optional later; JSONPath filter validation **rejects** unsupported filters.

## Consequences

### Positive

- Default path is safe for public docs and CI without secret-adjacent blobs.
- `version` / `appVersion` / checksum satisfy the primary inventory use case.
- Values remain available for enterprise audit when scope, sink, and redaction gates apply.
- Plain-Helm clusters have a documented secondary GVK once decode lands.

### Negative

- Flux-only primary sample does not cover raw `helm install` until Secret decode ships.
- Redaction is policy-first; operator scrub is follow-up work—authors must not add `spec.values`
  to public samples until scrub exists.
- `appVersion` alone may not reflect running app version in generic-chart GitOps; optional
  `image.tag` extraction adds profile complexity.

## Open questions

- **RESOLVED :** **`helm:release.<field>`** prefix on attribute paths; decoder expands
  `data.release` — Phase 2+ ([ADR-0201](0201-crd-model.md)).
- **RESOLVED :** Global **`scrubKeys[]`** operator config first; per-attribute `redact: true` later.
- **RESOLVED :** JSONPath filters — webhook **warn** Phase 1, **reject** Phase 2.
- **RESOLVED :** `chartVersion` from `status.lastAttemptedRevision`; `history[0]` ordering
  validated by contract test in CI.
