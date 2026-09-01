# ADR-0709: Separate the Helm chart and controller image into distinct GHCR paths

> Why the chart moves to `ghcr.io/platformrelay/charts/kollect` while the controller image stays
> at `ghcr.io/platformrelay/kollect`, and why no Artifact Hub setting can substitute for the move.

**Theme:** 07 · Project & meta · **Status:** Accepted (2026-08-19 — decided; amended 2026-09-01 — open decisions settled, execution authorised)

<!-- AgDR: architect role · 2026-08-19 · trigger: recurring Artifact Hub tracking-error mail -->
<!-- AgDR: architect role · 2026-09-01 · amendment: both open decisions settled after a fresh tracking mail added v0.19.0 to the error set -->

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

**3. Renaming the chart is ruled out.** Both sides of the coordinate force the last path segment
to stay `kollect`: `helm push` appends the chart name to the target it is given, and Artifact Hub
requires `oci://registry/namespace/chart-name`. A rename would therefore have to change the chart
name itself, and the chart name is what adopters type in `helm install`/`helm upgrade`.

> **Correction (2026-09-01).** The original text justified this with a second, stronger claim: that
> `_helpers.tpl` derives `app.kubernetes.io/name` from `.Chart.Name`, so a rename would mutate the
> immutable Deployment selector. **That is not true of this chart.**
> `charts/kollect/templates/_helpers.tpl:31-34` hardcodes the selector as literals:
>
> ```gotemplate
> {{- define "kollect.selectorLabels" -}}
> app.kubernetes.io/name: kollect
> control-plane: controller-manager
> {{- end }}
> ```
>
> No selector is at risk, and a rename would *not* break `helm upgrade`. The verdict stands on the
> naming constraint above, which is sufficient on its own. Recorded rather than silently edited
> because the false claim is the kind that gets cited later as evidence for an unrelated decision.

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

**Migration sequence — SUPERSEDED 2026-09-01 by the [migration runbook](#migration-runbook)
below. Do not execute the list in this subsection.** Its ordering instinct was right; two of its
details were wrong, and they are worth recording rather than quietly deleting.

<details markdown="1">
<summary>The 2026-08-19 sequence, and why it does not survive</summary>

> **Migration sequence** — each step is separately reversible, and the chart is published to the new
> path *before* anything is repointed:
>
> 1. Release workflow pushes the chart to `oci://ghcr.io/platformrelay/charts`, and `cosign sign`
>    targets the new reference. The `artifacthub-repo.yml` `oras push` moves to
>    `charts/kollect:artifacthub.io` — it sits in a **different job step** from the `helm push` and
>    is easy to miss. Without it, Verified Publisher breaks.
> 2. Repoint the Artifact Hub repository URL. `Manager.Update` keys on repository *name* and writes
>    a validated URL, so `repository_id`, stars and Verified Publisher status survive an in-place
>    edit. This is a control-panel action only the maintainer can perform.
> 3. Update the install coordinate across docs, README, and the install-docs test.
> 4. Leave the old bare tags in place. Once Artifact Hub no longer tracks
>    `ghcr.io/platformrelay/kollect` as a Helm repository, they are inert.

It did get the principle right — its preamble says the chart is published to the new path before
anything is repointed, and the runbook keeps exactly that. Two things are wrong underneath it, both
consequences of writing the sequence before Decision 1 was settled:

- **Nothing in it makes the new GHCR package public.** GHCR creates packages private by default, so
  following this list literally hands Artifact Hub a populated but *private* path — which reads as
  empty and costs the listing or the Verified Publisher badge. This is the failure most likely to
  be met in practice, and the whole sequence is silent about it. The runbook adds it as its own
  step for that reason.
- **Its step 1 populates the new path by cutting a release**, which is precisely the coupling
  Decision 1 rejects: it makes the fix wait for an unscheduled `0.20.0` while the tracking mail
  keeps arriving once per release. The runbook populates by copy instead, which is what lets the
  fix land on its own schedule.

Its preamble also claimed "each step is separately reversible". That is false of the URL repoint:
once the listing or the badge is gone, editing the URL back does not restore them.

Step 3 is carried into the runbook, moved later for the reason given there. Step 4 is not a step at
all — it is a standing fact, and it now lives where facts belong, under
[Explicitly rejected](#explicitly-rejected) and [Blast radius](#blast-radius).

</details>

**Open decisions** were deliberately left unsettled in the 2026-08-19 revision. Both are settled
in the amendment below and are no longer open.

## Amendment 2026-09-01 — both open decisions settled, execution authorised

**Trigger.** A fresh tracking mail listed `v0.9.0`–`v0.13.0` and, for the first time, **`v0.19.0`**
— the release cut 2026-08-31. That is the fact that changed the priority (DIST-AH-03 → P0): the
error set is not stable noise, it grows by **one permanent entry per release**. Impact remains
maintainer-facing only — `available_versions` has stayed correct at `0.14.0`–`0.19.0` throughout,
and no adopter sees a defect.

Re-verified the same day: the `artifacthub.io` metadata blob published at
`ghcr.io/platformrelay/kollect` is current (pushed 2026-08-31, byte-identical to this repository's
`artifacthub-repo.yml`) and the errors arrive regardless. That is a third independent confirmation
of mechanism 1 above, from live data rather than source reading. **The regex is still not the
lever.**

### Decision 1 — version history: COPY `0.14.0`–`0.19.0` across, by digest

Copy each published chart to `ghcr.io/platformrelay/charts/kollect` **preserving its digest**
(address it by tag if the tool prefers; the digest is what must survive), preferring
`cosign copy` (which carries the `sha256-<hex>.sig` signature tag with the artifact) and falling
back to a pair of `crane copy` calls if it does not. **`0.9.0`–`0.13.0` are never republished.** Four of them
(`0.9.0`, `0.10.0`, `0.11.0`, `0.13.0`) hardcode `image.tag: latest`, a tag that was never pushed,
so they cannot install. The fifth, `0.12.0`, is a different fault: it is the one release that pushed
the controller *image* to the chart's bare tag, so that coordinate never held a chart at all. Same
exclusion, two different reasons — and neither is fixable by republishing.

| criterion (weight) | **copy `0.14.0`–`0.19.0`** | copy `0.19.0` only | clean start at `0.20.0` |
| --- | --- | --- | --- |
| preserves published deep links (3) | 5 → 15 | 1 → 3 | 1 → 3 |
| fix lands without waiting on a release (3) | 5 → 15 | 5 → 15 | 2 → 6 |
| rollback stays inside one coordinate (2) | 5 → 10 | 5 → 10 | 5 → 10 |
| signature fidelity (2) | 4 → 8 | 4 → 8 | 5 → 10 |
| execution cost (1) | 5 → 5 | 5 → 5 | 4 → 4 |
| **total (max 55)** | **53** | 41 | 33 |

Three findings carried it:

- Repointing at a path holding only the newest chart **delists** `0.14.0`–`0.18.0`, so live
  `artifacthub.io/packages/helm/kollect/kollect/<version>` deep links stop resolving. This
  repository ships no versioned deep link of its own, so it is not literally the rule
  [ADR-0708](0708-operator-distribution-hubs.md) was amended on 2026-08-31 to enforce — that rule
  governs the URLs our docs publish. It is the same principle one step out: 0708 forbids shipping a
  URL that does not resolve, and this would break URLs *other people* already hold, which we cannot
  edit. Weaker footing than a direct rule violation; still the strongest of the three findings,
  because it is the only irreversible one.
- A clean start couples this fix to an unscheduled `0.20.0`, so the mail keeps arriving until then
  — and the whole point of the P0 is that each intervening release makes it worse.
- With history copied, rollback is "point the URL back"; without it, rollback would have to cross a
  coordinate boundary mid-incident.

**The signing objection that kept this decision open dissolved on inspection.** Every documented
verification command in this repository matches the signer with
`--certificate-identity-regexp '^https://github.com/platformrelay/kollect/.+'` and **none** pins a
workflow path: `cosign verify` at `docs/RELEASE.md:311,316`,
`docs/security/security-architecture.md:293` and `.github/release-notes-install.md:13,33`, and
`cosign verify-blob` at `docs/RELEASE.md:343`. The identity pattern is a repository prefix, so any
signature produced under `github.com/platformrelay/kollect/…` satisfies it.

That makes the fallback viable — but **only from a workflow**. A keyless re-sign has to run in
GitHub Actions with `id-token: write`, because that is what mints a Fulcio certificate whose SAN is
a `github.com/platformrelay/kollect/…` workflow URI. A maintainer re-signing interactively from a
laptop would get their own OIDC identity instead, which does **not** match the published pattern.
So the fallback is "add a one-shot backfill workflow", not "run cosign locally" — a real cost, and
the reason the fidelity score is 4 rather than 5.

Copying is preferred regardless, because it preserves each chart's original release-time Fulcio
identity instead of manufacturing a 2026-09-dated one for an artifact released months earlier.

Columns 1 and 2 score the same on signature fidelity, and deliberately so: both copy, so both carry
the same fallback cost — a one-shot backfill workflow — differing only in how many charts it loops
over. Scoring column 2 higher would have been the same error the matrix exists to prevent.

The matrix does not flip under the worst signing outcome either. If copied signatures do not verify,
column 1 loses six points — fidelity 4 → 1 at weight 2, total **53 → 47**. Column 2 is exposed to
exactly the same failure and falls by the same six, to **35**. Only column 3 is genuinely unaffected,
since a clean start signs natively at release time. So the ordering holds in every combination:
47 > 35 > 33, and 53 > 41 > 33 otherwise.

### Decision 2 — the `ignore` list: DELETE it at the new path

Remove the `ignore` key from `artifacthub-repo.yml`. **Keep `repositoryID` and `owners`** —
Verified Publisher requires both the matching repository ID and an owner email matching the
Artifact Hub account.

The regex would match nothing at the new path: there are no `v*` tags there, and `0.9.0`–`0.13.0`
are deleted and forbidden from republication. Carrying it forward would leave a rule whose only
effect is to mislead the next reader into thinking it is what suppresses the tracking mail.

Replace it with comments stating three things: the repository holds charts only; the DR-FIND-07
collision lives at the *other* path and is permanent there; and `ignore` filters **indexing**, never
**loading** — so adding a regex here to silence a future tracking error cannot work.

### V1 — the one empirical unknown, and it gates execution

Signature portability across the path boundary is reasoned from cosign/OCI semantics, not yet
observed. Run this **before** the bulk copy. A failure changes the tool, not the decision.

```sh
cosign copy -f ghcr.io/platformrelay/kollect:0.14.0 \
              ghcr.io/platformrelay/charts/kollect:0.14.0

# 1. the digest must be identical -- this is what "copy" has to mean here
crane digest ghcr.io/platformrelay/kollect:0.14.0
crane digest ghcr.io/platformrelay/charts/kollect:0.14.0

# 2. the published verify command must pass VERBATIM against the new path
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/platformrelay/kollect/.+' \
  ghcr.io/platformrelay/charts/kollect:0.14.0
```

Assert all three: identical digests, `cosign verify` exits 0, and the certificate SAN it prints is
the **original** `…/release.yaml@refs/tags/v0.14.0` rather than a freshly minted one. `cosign verify`
checks `critical.image.docker-manifest-digest`, not `critical.identity.docker-reference`, which is
why this is expected to pass — but expected is not observed.

Note which assertion carries which weight. The digest check alone does **not** prove a copy happened:
`helm push` of the same `.tgz` is deterministic, so a re-push would land on the same digest too. It
is the *certificate identity* that discriminates — an original release-time SAN can only have come
across with the artifact. And the copy is by tag at the command line only; what must be preserved,
and what assertion 1 actually checks, is the digest underneath it.

### Migration runbook

**Execution order is load-bearing.** The new path must be populated, public, and carrying metadata
*before* the Artifact Hub URL is repointed. Repointing at an empty, private, or metadata-less path
costs the listing or the Verified Publisher badge, and a URL edit is not a symmetric undo.

| # | step | who |
| --- | --- | --- |
| 1 | This amendment | harness |
| 2 | Release workflow derives the chart push, `cosign sign`, and the metadata `oras push` from **one** value; chart target becomes `charts/kollect` | harness |
| 3 | `artifacthub-repo.yml`: `ignore` deleted, `repositoryID` and `owners` kept; both hub gates tightened | harness |
| 4 | **V1** below | maintainer |
| 5 | `cosign copy` `0.14.0`–`0.19.0` to the new path | maintainer |
| 6 | **Set the new GHCR package public** — GHCR creates packages private by default | maintainer |
| 7 | `oras push …/charts/kollect:artifacthub.io` with the updated metadata | maintainer |
| 8 | Edit the Artifact Hub repository URL **in place** | maintainer |
| 9 | **Install coordinate updated across docs and README**, and the install-docs gate with it | harness |
| 10 | Verify per AC1 below | either |

**Why the docs repoint is step 9 and not step 3.** It is a repoint like any other: a `helm install`
line is a URL we ship, and `docs/**` publishes on push to `main` (`.github/workflows/docs.yaml`), so
merging it early puts an install command for an empty, private path in front of adopters. That is
the same defect as pointing the hub at one, and ADR-0708 forbids it directly. The work can be
*written* and reviewed at any time — it just must not *land* until step 6 has made the path real.

Steps 4–8 need a token carrying `write:packages` plus `cosign`/`crane`/`oras` on `PATH`; the
repository's own automation token has neither, so they cannot be run from CI or from a harness
session. Step 8 is a control-panel action with no API equivalent.

**Do not cut a release between steps 2 and 8.** Once step 2 has landed, the release workflow
publishes the chart *only* to the new path — while Artifact Hub is still tracking the old one. That
release would be invisible on the hub, and its `v`-prefixed image tag would add one more permanent
entry to the very error list this ADR exists to end. If a release becomes unavoidable mid-migration,
finish steps 4–8 first; none of them depends on cutting one.

**Never delete and re-create the Artifact Hub repository.** `Manager.Update` keys on repository
*name*, so an in-place URL edit preserves `repository_id`, stars, and Verified Publisher; a
delete/re-create loses all three.

### Verifying it worked (AC1)

`last_tracking_errors` is a **sample, not a census** — the reported set has changed between runs
with no corresponding registry change, which cost two earlier sessions a wrong conclusion. So
require all of: a recorded pre-repoint baseline; **two** reads with `last_tracking_ts` genuinely
advanced between them (an unchanged timestamp means the same run was sampled twice — the commonest
way to fake this result); both empty; **and** no tracking mail in the same window, as an
independent second witness.

### Explicitly rejected

**Dual-pushing the chart to both paths during a transition.** It re-creates the DR-FIND-07
collision in the image repository and re-arms the mail, defeating the entire ADR.

**Relaxing the DR-FIND-07 guard, or deleting the `docs/RELEASE.md` warnings, on the grounds that
"the chart moved."** Bare `0.14.0`–`0.19.0` stay in the image repository permanently, so
`crane digest ghcr.io/platformrelay/kollect:0.18.0` still returns a *chart* digest that passes
every string check. The trap is historical, not hypothetical — it has already shipped one defect —
and the manual-fallback runbook is exactly where someone will hit it.

### Blast radius

Nothing breaks for an adopter on day one: after the copy both paths serve byte-identical manifests
at identical digests. The sharpest risk is silent — anything *watching* the old coordinate (a
Renovate or Dependabot rule, an ArgoCD `Application`, a pinned CI job) keeps resolving and
installing, and simply never sees `0.20.0`. That is an announcement problem, not a code one.

**The OLM bundle is unaffected: zero bundle changes, no resubmission.** It references only the
controller image, by digest — verified across
`config/olm/template/manifests/kollect.clusterserviceversion.yaml` (lines 275, 524 and 628 are the
image; line 314 is prose carrying no URL).

## Retained and not addressed

**Retained:** the DR-FIND-07 collision guard in the release workflow stays. Separate paths make a
collision unreachable, but the guard costs nothing and fails closed.

**Not addressed:** the four `image not found … :latest` scanner errors, which come from charts
`0.9.0`–`0.13.0` that shipped an `image.tag: latest` never pushed to GHCR. Those charts are
deleted; the errors are stale output from a scanner run predating the deletion.
