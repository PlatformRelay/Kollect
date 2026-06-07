# ADR-0104: Security model — secrets, TLS, RBAC, and redaction

> The consolidated threat model and security posture: how credentials, TLS trust, least-privilege RBAC,
> and payload redaction are handled across Kollect.

**Theme:** 01 · Foundations · **Status:** Current

## Context

Security decisions were spread across many ADRs — TLS trust ([ADR-0201](0201-crd-model.md)), namespaced
isolation and SAR ([ADR-0203](0203-namespaced-multi-tenancy.md)), redaction ([ADR-0303](0303-helm-release-inventory.md)),
HTTP/API auth ([ADR-0404](0404-inventory-api-auth.md)), hub mTLS ([ADR-0503](0104-security-model.md)) —
but there was no single model a reviewer could read to understand Kollect's posture. This ADR
consolidates it. (`SECURITY.md` at the repo root remains the *disclosure policy*; this is the
*architecture*.)

### Threat model (what we defend against)

- A compromised or misconfigured tenant reading/exporting resources outside its namespace.
- Secrets (registry creds, DB passwords, tokens, kubeconfigs) leaking into exported inventory,
  logs, or etcd status.
- Untrusted/MITM'd sink or cluster endpoints.
- Over-broad operator RBAC enabling privilege escalation.

## Decision

### Secret handling

- Credentials are referenced by `secretRef`, never inlined in CRD specs ([ADR-0201](0201-crd-model.md)).
- Reconcilers resolve secrets and pass material via `BuildContext` (`SecretData`, `DatabaseSecretData`,
  `CAPEM`) — backends never read Kubernetes secrets themselves ([ADR-0406](0406-sink-registry.md)).
- Secret values are never logged and never written to CR `status` or events ([ADR-0602](0602-error-taxonomy.md)).

### TLS trust

- Sinks and cluster connections trust a configurable CA (`caPEM`) resolved from a secret/configmap.
- `insecureSkipVerify` exists only where unavoidable (HTTP-ish backends), is **off by default**, and is
  a webhook-warned/loud opt-in. **Git/object-store and hub transport require real trust** — no skip.

### RBAC (least privilege)

- The operator's `ClusterRole` grants only the verbs needed (watch/list/get on selected GVKs; CRUD on
  Kollect CRDs; leader-election on a single lease).
- **Tenant mode** ([ADR-0203](0203-namespaced-multi-tenancy.md)) narrows watches to configured namespaces;
  chart emits `Role`/`RoleBinding` instead of cluster-wide bindings ([ADR-0704](0704-helm-chart-crd-lifecycle.md)).
- Cross-namespace reads in namespaced inventories are **SubjectAccessReview-gated**: missing permission
  degrades gracefully (skip + condition) rather than escalating.

### Redaction (no secrets in payloads)

- Profiles redact via `scrubKeys`/redaction before items enter the store ([ADR-0303](0303-helm-release-inventory.md)),
  so the export data contract ([ADR-0405](0405-export-data-contract.md)) is secret-free by construction.
- Helm release values, Secret data, and known-sensitive keys are scrubbed at extraction time, not at
  export time.

### API / HTTP exposure

- The read API requires auth (token/mTLS) and is namespace-scoped ([ADR-0404](0404-inventory-api-auth.md));
  it is a feature-gated, off-by-default surface.

### Multi-cluster

- Spoke→hub uses mTLS / mesh identity ([ADR-0503](0104-security-model.md)); the hub
  allowlists clusters and rejects unlisted ones.

### Webhook TLS (serving)

- Validating webhook serving certs: cert-manager default, self-signed bootstrap fallback
  ([ADR-0105](0105-webhook-serving-cert-management.md)).

### Supply chain

- Images are signed and SBOM'd; see [ADR-0705](0705-release-supply-chain.md).

## Consequences

- A reviewer has one page for the posture; individual ADRs hold the detail.
- Redaction-at-extraction means a sink can never accidentally export a secret the store never held.
- SAR-gated degradation favors availability + safety over hard failure on partial permissions.

## Open questions

- **DECIDED :** Encryption-at-rest for sinks (Postgres/object-store) is **recommended and
  documented**, not enforced by the operator (it's a backend/infra responsibility).
- **DECIDED :** Add a **formal RBAC audit gate in CI** (`kubeaudit`-style) as a maturity
  signal ([ADR-0705](0705-release-supply-chain.md)).
- **OPEN:** A built-in secret-leak scanner over outgoing payloads as defense-in-depth beyond `scrubKeys`?
