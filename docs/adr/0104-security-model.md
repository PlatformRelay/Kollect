# ADR-0104: Security model — secrets, TLS, RBAC, and redaction

> The consolidated threat model and security posture: how credentials, TLS trust, least-privilege RBAC,
> and payload redaction are handled across Kollect.

**Theme:** 01 · Foundations · **Status:** Current

## Context

Security decisions were spread across many ADRs — TLS trust ([ADR-0201](0201-crd-model.md)), namespaced
isolation and SAR ([ADR-0203](0203-namespaced-multi-tenancy.md)), redaction ([ADR-0303](0303-helm-release-inventory.md)),
and HTTP/API auth ([ADR-0404](0404-inventory-api-auth.md)) — but there was no single model a reviewer
could read to understand Kollect's posture. This ADR records the decision; the maintained
[security architecture](../security/security-architecture.md) maps it to current controls.
(`SECURITY.md` at the repo root remains the disclosure policy.)

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
- Secret values are not intentionally logged or written to CR `status` or Events. Known credentials
  are sanitized in Git errors, but generic third-party backend error strings do not yet have a
  centralized sanitizer ([ADR-0602](0602-error-taxonomy.md)).

### TLS trust

- Sinks and cluster connections trust a configurable CA (`caPEM`) resolved from a secret/configmap.
- `insecureSkipVerify` exists only where unavoidable (HTTP-ish backends), is **off by default**, and is
  an explicit compatibility opt-in. It weakens server identity and is independent of private-address
  reachability.
- Git HTTPS/SSH retains server-name or host-key verification while using the resolved-address guard.

### RBAC (least privilege)

- The operator's `ClusterRole` grants only the verbs needed (watch/list/get on selected GVKs; CRUD on
  Kollect CRDs; leader-election on a single lease).
- **Tenant mode** ([ADR-0203](0203-namespaced-multi-tenancy.md)) narrows watches to configured namespaces;
  chart emits `Role`/`RoleBinding` instead of cluster-wide bindings ([ADR-0704](0704-helm-chart-crd-lifecycle.md)).
- Cross-namespace reads in namespaced inventories are **SubjectAccessReview-gated**: missing permission
  degrades gracefully (skip + condition) rather than escalating.

### Redaction and data minimization

- Profiles redact via `scrubKeys`/redaction before items enter the store ([ADR-0303](0303-helm-release-inventory.md)),
  reducing what can reach the export data contract ([ADR-0405](0405-export-data-contract.md)).
- Helm release values, Secret data, and known-sensitive keys are scrubbed at extraction time, not at
  export time. Key-based scrubbing cannot classify every business-sensitive field.

### API / HTTP exposure

- The read API is feature-gated and off by default. Its default mode uses Kubernetes bearer
  TokenReview + SubjectAccessReview and is namespace-scoped
  ([ADR-0404](0404-inventory-api-auth.md)); a disabled-auth mode exists for local development.
- The built-in listener is plain HTTP. Deployers provide transport confidentiality through ingress,
  a service mesh, a local port-forward, or an equivalent network boundary.

### Outbound network policy

- Sink endpoints are checked at admission when default-on webhooks are enabled and again after DNS
  resolution at dial time
  ([resolved-address policy](../security/resolved-address-policy.md)).
- Private RFC1918/ULA sinks are denied by default. The cluster-admin Helm value
  `allowPrivateSinks` renders `--allow-private-sinks`; it is not a CRD or tenant field.
- Loopback, link-local/cloud metadata, unspecified, multicast, carrier-grade NAT, benchmark ranges,
  and known metadata hostnames remain denied when the private-sink opt-in is enabled. Default-on
  admission also rejects `file://`; disabling webhooks removes that check, and runtime Git accepts
  valid local bare-repository paths.
- The chart does not ship a sink-aware egress `NetworkPolicy`; deployers constrain egress to the
  Kubernetes API, DNS, and approved sinks.

### Multi-cluster

- Each cluster runs its own manager and exports to shared sinks partitioned by `spec.cluster`
  ([ADR-0501](0501-multi-cluster-fleet.md)). There is no privileged central hub transport.

### Webhook TLS (serving)

- Validating webhooks are enabled by default and the chart provisions their serving certificate
  through cert-manager. Disabling chart-managed certificate resources requires operator-provided
  serving certificates and CA injection; the self-signed fallback described in
  [ADR-0105](0105-webhook-serving-cert-management.md) is not implemented.

### Supply chain

- Operator, pipeline, and UI images are scanned, signed, SBOM'd, and published with provenance; see
  [ADR-0705](0705-release-supply-chain.md).

## Consequences

- A reviewer has one page for the posture; individual ADRs hold the detail.
- Redaction-at-extraction prevents sinks from exporting fields removed before they enter the store;
  profile authors must still classify and scrub business-sensitive values.
- SAR-gated degradation favors availability + safety over hard failure on partial permissions.
- Private-sink reachability is an explicit cluster-operator trade-off rather than tenant-controlled
  configuration.

## Open questions

- **DECIDED :** Encryption-at-rest for sinks (Postgres/object-store) is **recommended and
  documented**, not enforced by the operator (it's a backend/infra responsibility).
- **DECIDED :** Add a **formal RBAC audit gate in CI** (`kubeaudit`-style) as a maturity
  signal ([ADR-0705](0705-release-supply-chain.md)).
- **OPEN:** A built-in secret-leak scanner over outgoing payloads as defense-in-depth beyond `scrubKeys`?
