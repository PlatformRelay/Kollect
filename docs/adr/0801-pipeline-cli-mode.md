# ADR-0801: Pipeline CLI mode — collection without operator deployment

> Run the Kollect collection and snapshot-export path from a kubeconfig and local CR-shaped YAML.

**Theme:** 08 · Pipeline & CLI · **Status:** Accepted (implemented)

## Context

CI jobs and evaluators often have Kubernetes read access but cannot install cluster-scoped CRDs,
webhooks, or a long-running operator. They still need the same extraction and snapshot semantics as
an in-cluster Kollect installation.

## Decision

Kollect ships the `kollect-pipeline` binary and container image. Its `collect` command:

- loads `KollectProfile`, `KollectTarget`, an optional `KollectSnapshotSink`, and referenced Secret
  manifests from a local configuration directory;
- builds a dynamic client from `--kubeconfig` and selected `--context` names or globs;
- lists matching Kubernetes resources once, then uses the shared extractor and export contracts;
- writes deterministic files with `--output`, or exports through one configured Git, GitLab, S3,
  or GCS snapshot sink;
- resolves exact `${env:NAME}` Secret value placeholders from the process environment;
- writes data to stdout only when the explicit stdout output mode is selected, while diagnostics
  stay on stderr; and
- returns documented success, partial-failure, and fatal-configuration exit codes.

The CLI never installs CRDs and never writes to the observed cluster. Local manifests use the same
API types as operator resources, so configuration can later be applied to a cluster when an operator
installation is available.

## Architecture

The CLI reuses `api/v1alpha1`, extraction, canonical inventory serialization, sink registry, network
guards, and backend implementations. Pipeline-specific code owns manifest loading, kubeconfig and
context selection, one-shot list orchestration, local/stdout output, and exit-code aggregation. It
does not import controllers, webhook registration, informer lifecycle, leader election, or manager
wiring; architecture tests enforce that boundary.

Runs across multiple contexts are deterministic and isolated: context selections are de-duplicated
and sorted, each context completes its collect/export pass, and the process returns the most severe
result. When `spec.cluster` is absent, the context name supplies the cluster partition.

## Security boundary

- Kubernetes RBAC on the supplied identity bounds observable resources.
- Sink endpoint and TLS protections are shared with operator mode.
- Credentials come from local Secret manifests and environment placeholders, never command-line
  literal flags.
- `KollectScope` is not evaluated because the CLI does not read Kollect objects from the cluster;
  CI ownership and Kubernetes RBAC are the policy boundary.

## Consequences

- Users can trial Kollect and produce diffable artifacts without cluster installation privileges.
- One-shot lists trade informer efficiency and event-driven freshness for a small, auditable CI
  footprint.
- The release publishes and signs a second image alongside the operator image.
- Database and event sinks remain operator-mode surfaces; pipeline mode intentionally accepts at
  most one snapshot destination.

Operational examples and the exact flag contract live in the [Pipeline CLI guide](../guides/pipeline-cli.md).
