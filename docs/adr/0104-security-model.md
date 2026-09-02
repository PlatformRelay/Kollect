# ADR-0104: Security model — secrets, TLS, RBAC, and redaction

> The consolidated threat model and security posture: how credentials, TLS trust, least-privilege RBAC,
> and payload redaction are handled across Kollect.

**Theme:** 01 · Foundations · **Status:** Current (see the 2026-09-02 note below)

<!-- AgDR: implementer role · 2026-09-02 · amendment: SEC-SSHHOSTKEY-01 — the TLS-named insecureSkipVerify also disables SSH host-key verification -->

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
  reachability. When set, reconcilers surface status condition `TLSInsecure`
  (`ConditionTLSInsecure`) so operators can see the opt-in without reading the spec.
- Git HTTPS/SSH retains server-name or host-key verification while using the resolved-address guard.
  *Amended 2026-09-02:* this is true **only while `spec.tls.insecureSkipVerify` is unset**, which is
  the default. The flag is transport-scoped, not TLS-scoped — see
  *`insecureSkipVerify` is transport-scoped — corrected 2026-09-02* below.

### `insecureSkipVerify` is transport-scoped — corrected 2026-09-02

The bullet above read as an unconditional claim, and it is not one. `spec.tls.insecureSkipVerify`
is named for TLS but disables verification of the remote's identity for **whichever transport the
sink's endpoint selects**. For an `https://` git remote it skips server-certificate verification;
for an `ssh://` git remote it installs `ssh.InsecureIgnoreHostKey` on the go-git path
(`internal/sink/git/ssh_auth.go`) and `StrictHostKeyChecking=no` on the git-CLI path
(`internal/sink/git/cli_env.go`, which the connection test also goes through). This behaviour is
pre-existing, not a regression; what was missing was any sentence saying so.

**Why the escape hatch is one flag and not two.** A sink has exactly one `spec.endpoint`, and the
transport is chosen from that endpoint's scheme (`buildAuthMethod` in `internal/sink/git/auth.go`
switches on it and rejects a mismatched `spec.git.auth.type`). HTTPS and SSH are therefore mutually
exclusive per sink, and "skip verification of the remote's identity for this sink's transport" is a
single coherent contract. Splitting it into a second CRD field would add a public API surface with
defaulting and migration cost to express a choice the operator cannot make independently.

**Guard rails, with their conditions stated:**

- **Off by default** and settable only by whoever can write the sink spec.
- **Surfaced in status**: the `TLSInsecure` condition is set whenever `spec.tls.insecureSkipVerify`
  is true, for every sink family. Its current message names TLS only.
- **Fails closed on the go-git path**: without the flag and without a `known_hosts` key in the
  referenced secret, `sshAuthMethod` returns an error rather than falling back to the system
  `known_hosts` or to trust-on-first-use.
- **The git-CLI path has no equivalent fail-closed guard.** With the flag unset and no `known_hosts`
  supplied it simply omits `UserKnownHostsFile` and leaves host-key policy to the ambient ssh
  configuration. It never sets `StrictHostKeyChecking=no` unless the flag is set, but the outcome on
  an unknown host is then ssh's default, not a Kollect decision. Supply `known_hosts` when using the
  CLI engine over SSH.
- The resolved-address guard (`pinGoGitSSHResolution`, NetGuard) is unaffected by the flag: it
  still dials the checked numeric address, and still passes the original hostname for verification.
  When the flag is set there is simply nothing left to verify the hostname against.
- `internal/sink/git/insecure_hostkey_contract_test.go` pins all of the above so the code and this
  ADR cannot drift apart again.

Treat it as a temporary development exception: prefer supplying `known_hosts` (or a trusted CA for
HTTPS), and record the reason and expiry for any sink that sets it.

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
  serving certificates and CA injection; the chart does not generate TLS material on that path
  ([ADR-0105](0105-webhook-serving-cert-management.md)).

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
