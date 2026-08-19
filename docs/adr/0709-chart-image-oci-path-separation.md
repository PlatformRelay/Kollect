# ADR-0709: Separate the Helm chart and controller image into distinct GHCR paths

> Why the chart moves to `ghcr.io/platformrelay/charts/kollect` while the controller image stays
> at `ghcr.io/platformrelay/kollect`, and why no Artifact Hub setting can substitute for the move.

**Theme:** 07 · Project & meta · **Status:** Proposed (2026-08-19)

<!-- AgDR: architect role · 2026-08-19 · trigger: recurring Artifact Hub tracking-error mail -->

## Context

[ADR-0708](0708-operator-distribution-hubs.md) records the current GHCR layout, known as
**DR-FIND-07**: a single OCI repository holds two artifact kinds.

| coordinate | artifact |
| --- | --- |
| `ghcr.io/platformrelay/kollect:0.18.0` (bare tag) | Helm chart |
| `ghcr.io/platformrelay/kollect:v0.18.0` (v-prefixed) | controller image |

This tag convention is load-bearing but invisible: both coordinates resolve to a valid
`sha256:` digest, so a wrong-tag reference passes every format check. It has already produced
one shipped defect — the OLM bundle pinned the **chart** digest as the controller image, which
OLM surfaced only as a `CreateContainerError` at install time.

It also produces a standing operational cost: Artifact Hub mails the maintainer
`error preparing package: error loading chart (oci://ghcr.io/platformrelay/kollect:v0.10.0):
layer not found` on every tracking run, once per historical `v*` tag.

## Why no Artifact Hub setting fixes this

Three independent mechanisms were checked against upstream source and documentation, not inferred
from observed behaviour. All three are dead ends, and each was tried or considered first.

**1. The `ignore` list cannot suppress these errors.** `internal/tracker/source/helm/helm.go`
lists every tag and passes each to `preparePackage()`, which downloads the artifact; the
`error preparing package` warning is emitted from that failure. The repository `ignore` list
filters the *result set*, so it runs strictly after the load. Upstream wording agrees — ignore
covers "packages that should not be **indexed**".

This precisely explains the observations that cost two earlier sessions: bare `0.9.0`–`0.13.0`
are correctly absent from `available_versions` (indexing *is* filtered) while their load errors
keep arriving (loading is *not*). **The regex is not the lever. Do not tune it again.**

**2. Artifact Hub deliberately supports one chart per repository entry.** Maintainer `tegioz` in
[discussion #3632](https://github.com/artifacthub/hub/discussions/3632): *"the OCI distribution
spec does not define a mechanism to list all references for a given namespace. Artifact Hub needs
an entry point to start indexing content."* The documented URL format is
`oci://registry/namespace/chart-name`. There is no allowlist, no per-tag type hint, no media-type
filter — by design, not as a missing feature.

**3. Renaming the chart is ruled out.** `charts/kollect/templates/_helpers.tpl` derives names from
`.Chart.Name`, which feeds `app.kubernetes.io/name` and therefore the Deployment selector.
Selectors are immutable, so a rename breaks `helm upgrade` for every existing install.

## Options considered

| # | Option | Verdict |
| --- | --- | --- |
| 1 | Status quo — one repo, two tag conventions | Rejected: recurring mail, and a proven defect class |
| 2 | Tune the `ignore` regex | **Impossible** — filters indexing, not loading (above) |
| 3 | Move the controller image to `kollect-controller` | Rejected: breaks published immutable references |
| 4 | Give the image a non-semver tag (`img-0.18.0`) | Rejected on durability — see below |
| 5 | **Move the chart to `charts/kollect`** | **Accepted** |

### Option 4 deserves its own note, because it does work

`internal/oci/oci.go` filters tags with `semver.NewVersion(tag)` before any load. Masterminds
semver accepts a leading `v`, which is exactly why `v0.18.0` is picked up and loaded as a chart.
A tag that does not parse as semver — `img-0.18.0` — is dropped before `preparePackage()` and
would silence the mail with no path change, no Artifact Hub edit, and no loss of version history.
It is genuinely cheaper.

It is rejected because it depends on **undocumented internal behaviour**. Nothing upstream
promises that filter stays, and a future release that relaxes it or normalises tag prefixes brings
the mail back — with the controller image then sitting on a non-idiomatic tag scheme for nothing.
Option 5 works because Artifact Hub's design *requires* one chart per repository: a documented
contract with a maintainer statement behind it. Prefer the contract over the implementation detail.

## Decision

Publish the Helm chart to **`ghcr.io/platformrelay/charts/kollect`**. The controller image stays
at `ghcr.io/platformrelay/kollect` with its `v`-prefixed tags, unchanged.

**The controller image must not move.** Its digest is pinned immutably in OLM bundles already
merged into `community-operators` and `community-operators-prod`, and its cosign signatures are
bound to the current path. Those references are effectively permanent. The chart's coordinate, by
contrast, appears only in a `helm install` line.

**This is not a second GitHub repository.** Source, issues, CI, releases and the OLM bundle all
stay in `platformrelay/kollect`. A GHCR package is a registry path; several packages link to one
source repository. The last path segment must remain `kollect` because both sides force it —
`helm push` appends the chart name, and Artifact Hub requires `…/chart-name`. Nested paths are
supported and are the common layout (`ghcr.io/nginxinc/charts/nginx-ingress`).

## Consequences

**Migration sequence** — each step is separately reversible, and the chart is published to the new
path *before* anything is repointed:

1. Release workflow pushes the chart to `oci://ghcr.io/platformrelay/charts`, and `cosign sign`
   targets the new reference. The `artifacthub-repo.yml` `oras push` moves to
   `charts/kollect:artifacthub.io` — it sits in a **different job step** from the `helm push` and
   is easy to miss. Without it, Verified Publisher breaks.
2. Repoint the Artifact Hub repository URL. `Manager.Update` keys on repository *name* and writes
   a validated URL, so `repository_id`, stars and Verified Publisher status survive an in-place
   edit. This is a control-panel action only the maintainer can perform.
3. Update the install coordinate across docs, README, and the install-docs test.
4. Leave the old bare tags in place. Once Artifact Hub no longer tracks
   `ghcr.io/platformrelay/kollect` as a Helm repository, they are inert.

**Open decisions, deliberately not settled here:**

- **Version history.** Starting the new path at `0.19.0` loses the `0.14.0`–`0.18.0` listing on
  Artifact Hub. Copying history across preserves it, but charts are signed against the old
  reference — whether copied signatures still verify must be tested before choosing, not assumed.
- **The `ignore` list.** The current regex would keep suppressing `0.9.0`–`0.13.0` in the new
  repository. If history is not copied it becomes dead weight; if it is, it is now suppressing
  those versions *on purpose* rather than as leftover cleanup. Either way, restate the intent.

**Retained:** the DR-FIND-07 collision guard in the release workflow stays. Separate paths make a
collision unreachable, but the guard costs nothing and fails closed.

**Not addressed:** the four `image not found … :latest` scanner errors, which come from charts
`0.9.0`–`0.13.0` that shipped an `image.tag: latest` never pushed to GHCR. Those charts are
deleted; the errors are stale output from a scanner run predating the deletion.
