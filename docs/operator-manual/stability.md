# Stability and support

Kollect is a **public alpha**. Its API is `kollect.dev/v1alpha1`, it is maintained by one person,
and it changes between releases. This page says what can change, how you hear about it, and which
versions get fixes.

## API version

Every Kollect CRD is served and stored as `v1alpha1`. There is a single version and **no
conversion webhook**, so a schema change takes effect when you apply the new CRDs. A `v1beta1` API
with a conversion window is planned but not scheduled
([planned features](../roadmap/planned-features.md#api-v1beta1-and-conversion-webhook),
[ADR-0206](../adr/0206-api-versioning-conversion.md)).

## What may change between minor releases

While the API is `v1alpha1`, a minor release (`0.X.0`) may change, without a prior deprecation
release:

- CRD fields, enum values, defaults and validation rules;
- Helm chart values and their defaults;
- the export layout and content: paths, file names, annotations and bookkeeping files in Git,
  object keys, table columns and event payloads;
- status conditions and reasons, Prometheus metric names and labels, and `kollect-pipeline` flags
  and exit codes;
- which errors are retried and which are terminal.

There is no deprecation period while the API is `v1alpha1`: a removal can ship in the next minor
release. Deprecation guarantees start with `v1beta1`.

## How breaking changes are announced

Every breaking change ships in a minor release and is announced in two places:

- the [upgrade guide](upgrading.md), with a section that says what changed, how to find affected
  objects before you upgrade, and what to do;
- the upgrade notes at the top of the
  [GitHub Release](https://github.com/platformrelay/kollect/releases), with the same remedy.

The generated changelog lists commits, and not every breaking change is flagged there. Read the
upgrade guide. When you skip minor releases, apply the notes of every release in between.

## Versioning

Kollect follows the rule in the [release process](../RELEASE.md#versioning-policy):

| Release | May contain |
| --- | --- |
| Minor (`0.X.0`) | New features, and breaking API, default or behaviour changes |
| Patch (`0.X.Y`) | Fixes only. Compatible with the other releases of its minor line |
| Release candidate (`0.X.0-rc.N`) | A pre-release for testing. Not supported |

## Supported versions

Only the **latest minor release** is supported. Fixes, including security fixes, land on `main`
and ship in the next release; there are no backports to older minors. If you report a problem on
an older release, the first answer is to upgrade.

The chart accepts Kubernetes 1.28 and later (`kubeVersion` in `Chart.yaml`). CI runs its kind
scenarios on the Kubernetes version that matches the client libraries the release is built with
(`k8s.io/api` in `go.mod`). Older Kubernetes versions are accepted but not tested.

## Support

Support is best effort. Ask questions and report bugs in
[GitHub issues](https://github.com/platformrelay/kollect/issues); there is no response-time
commitment. Report security problems privately, as [SECURITY.md](https://github.com/platformrelay/kollect/blob/main/SECURITY.md)
describes, not in a public issue.

## Maturity of sink types

Each sink type carries a maturity tier (Core, Beta or Planned) on the [roadmap](../ROADMAP.md).
Beta sinks may change more between minor releases than Core ones, but the policy on this page
applies to all of them.
