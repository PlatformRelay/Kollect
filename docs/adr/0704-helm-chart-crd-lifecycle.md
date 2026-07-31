# ADR-0704: Helm chart and CRD lifecycle

> How Kollect is packaged, installed, and upgraded without relying on Helm to upgrade CRDs.

**Theme:** 07 · Project & meta · **Status:** Current

## Decision

The OCI Helm chart contains generated CRDs under `crds/`, the controller Deployment, service
account, cluster-wide or tenant RBAC, leader election, metrics, and validating-webhook resources.
Values select cluster-wide versus restricted namespace watches and optional read/debug surfaces.

### Webhook certificates

Validating webhooks are enabled by default. The default chart renders cert-manager `Issuer` and
`Certificate` resources, so cert-manager is a prerequisite. Setting
`webhooks.certManager.create: false` suppresses those resources only; the installer must provide the
named TLS Secret and webhook CA trust through another PKI workflow. The chart does not generate
certificates on that path. Development overlays may disable webhooks explicitly.

### CRD lifecycle

Helm installs files in `crds/` on first install but does not upgrade or delete them. Kollect
therefore publishes two artifacts:

1. `dist/install-crds.yaml`, applied explicitly before an operator upgrade.
2. The OCI chart, upgraded after the CRD schema is current.

Automation never deletes CRDs because deletion also deletes every custom resource. Downgrades must
respect stored-version compatibility and are not implemented as CRD replacement.

### Distribution and verification

The chart and controller image are published to GHCR by the tag-driven release workflow. Chart
schema validation, Helm lint, unit snapshots, generated README verification, and release provenance
are merge/release gates.

## Consequences

- Upgrades are deliberately two-step, making schema changes explicit and reviewable.
- The secure default gains automated certificate rotation but requires cert-manager.
- Alternate PKI integrations are possible, with Secret lifecycle and CA injection owned by the
  operator of that integration.
