# ADR-0708: Operator distribution via Artifact Hub and OperatorHub

> How Kollect becomes discoverable on Artifact Hub and OperatorHub without replacing Helm OCI
> as the primary install path, and without in-repo FBC/`opm` catalog machinery.

**Theme:** 07 · Project & meta · **Status:** Current (accepted 2026-08-08 — see the 2026-08-18 and 2026-08-31 notes below)

<!-- AgDR: architect role · 2026-08-06 · trigger: hub distribution parity plan (Attune pattern) -->

## Context

Kollect already publishes signed multi-arch images and a signed Helm OCI chart to GHCR
([ADR-0705](0705-release-supply-chain.md)), with CRD lifecycle rules in
[ADR-0704](0704-helm-chart-crd-lifecycle.md). Chart metadata already carries
`artifacthub.io/license: MIT` but lacks operator/CRD/capability annotations and Verified
Publisher OCI metadata. There is no OLM bundle or OperatorHub submission path.

Attune (reference) demonstrates lean hub parity: Chart.yaml annotations, root
`artifacthub-repo.yml` pushed via `oras` as `:artifacthub.io`, hand-templated registry+v1
bundles, and dual community-operators PRs from release — soft-fail so core publish stays green.

**Note on GHCR layout (DR-FIND-07):** the Helm chart occupies
`ghcr.io/<owner>/kollect:<version>` (bare tag); the controller image uses the **v-prefixed**
tag only. Artifact Hub Verified Publisher metadata must target the **chart** OCI repository
(`…/kollect:artifacthub.io`), never clobber image tags.

Package identity: **`kollect`**, channel **`stable`**.
`kollect-versions` / `kollect-render` are out of scope.

## Options considered

| Option | Pros | Cons |
| --- | --- | --- |
| **A. Full Attune parity** (AH + OLM hand bundle + dual PRs) | Discoverability on both hubs; reuses release pipeline | Hand CSV drift risk; registration prerequisites |
| **B. Artifact Hub only** | Smaller; Helm already primary | No OpenShift / OperatorHub.io path |
| **C. operator-sdk generate + in-repo FBC** | Toolchain-native OLM | Heavy vs lean release engineering; FBC ops |
| **D. Do nothing** | Zero cost | Hubs stay dark after launch docs investment |

### Weighted trade-off (subjective scores 1–5)

| Criterion (weight) | A | B | C | D |
| --- | ---: | ---: | ---: | ---: |
| Adopter discoverability (3) | 5 | 3 | 5 | 1 |
| Operability / lean tooling (3) | 4 | 5 | 2 | 5 |
| Continuity with ADR-0704/0705 (2) | 5 | 5 | 3 | 3 |
| Release blast radius (soft-fail) (2) | 4 | 5 | 3 | 5 |
| Pattern familiarity (Attune) (1) | 5 | 3 | 2 | 1 |
| **Weighted total** | **55** | **46** | **35** | **35** |

## Decision

We chose **option A — full Attune-style Artifact Hub + OperatorHub distribution** because it
extends ADR-0705 without replacing Helm OCI, accepting hand-templated OLM bundles and
operator-owned registration, over B (incomplete), C (tooling weight), or D (no discoverability).

Contract:

1. **Helm OCI remains primary** ([ADR-0704](0704-helm-chart-crd-lifecycle.md)).
2. **Artifact Hub:** enrich `charts/kollect/Chart.yaml` annotations (`operator`, `category`,
   `operatorCapabilities`, `crds`, `links`, `images`, keep `license`); root
   `artifacthub-repo.yml` with placeholder `repositoryID` until registration; release
   `oras push ghcr.io/<owner>/kollect:artifacthub.io` after helm push + cosign.
3. **OperatorHub:** `config/olm/` templates + `make generate-olm-bundle` copying CRDs from
   `config/crd/bases`; package **`kollect`**, channel **`stable`**; digest-pin CSV
   deployment/`relatedImages` to `ghcr.io/<owner>/kollect@sha256:…` (image digest from release,
   not chart digest).
4. **RBAC in CSV** from real `config/rbac/role.yaml` (+ leader-election as needed), gated
   against drift by `hack/test/dist_olm_bundle_test.sh`.
5. **`hack/operatorhub-pr.sh`** + release job gated on `secrets.OPERATORHUB_PAT`, soft-fail
   (`continue-on-error`); OpenShift annotation default **`v4.19`**; fork owner default
   **`platformrelay`**. The job runs in the protected **`release`** environment — the PAT is a
   cross-repo write credential and must not be reachable without the eligibility gate — and
   checks out the eligibility-proven SHA, never the mutable tag ref.
6. **Soft-fail must not be silent, and hub steps run last.** Every hub step sits *after*
   `Publish GitHub Release` so no hub failure can strand a tag with signed GHCR artifacts and
   no release, and each reports through `steps.<id>.outcome` (`continue-on-error` pins
   `.conclusion` to `success`) with a `::warning::` annotation and a `$GITHUB_STEP_SUMMARY`
   line.
7. **Skipped:** Krew, Docker Hub chart mirror, in-repo FBC/`opm` (posture re-confirmed
   2026-08-18 against the upstream recommendation — see *Alternatives considered*), CLOMonitor.

## Consequences

- Hub wiring can land before registration using placeholder repositoryIDs; live Verified
  Publisher and upstream PRs wait on operator registration.
- CSV must list owned CRDs Kollect actually ships (Profile, Target, Inventory, sink families,
  scopes, connection tests, cluster variants) — large owned list; keep generate step mechanical.
- Docs must not ship hub badge or prose URLs that do not resolve to a **live listing**. *Amended
  2026-08-31:* this originally read "…that 404 before listing", and that wording is what let a dead
  OperatorHub.io link ship — see *Badge URLs — corrected 2026-08-31* below. An HTTP 200 that
  client-renders "can't find package …" is a dead link; the status code is not the test.
- Soft-fail hub jobs preserve tag-release success when PAT/forks are absent.
- **Shipped 2026-08-18.** The contract above is live, which is what moves this ADR to *Current*:
  [#309](https://github.com/platformrelay/kollect/pull/309) merged (`e71faaffd`); Artifact Hub
  repository `kollect` registered with the real `repositoryID` and **Verified Publisher active**;
  both community-operators submissions open and green. The operator accepted option A on
  2026-08-08, so the placeholder-`repositoryID` and pre-registration caveats above are history,
  not open work.

### Badge URLs — corrected 2026-08-31

When [#309](https://github.com/platformrelay/kollect/pull/309) shipped on 2026-08-18 it carried an
OperatorHub.io **package deep link** in the README badge header and in the README prose, ahead of the
listing. That was an explicit operator override of the *Docs must not ship…* consequence above, and
it rested on a premise that has since been checked and found false:

> operatorhub.io soft-404s — it serves HTTP 200 with the generic landing page for unknown operators,
> so a premature badge degrades gracefully rather than showing a broken link.

It does not. The package URL answers **HTTP 200** and then **client-renders "can't find package
kollect"** — a broken-looking dead end, which is the precise user-visible outcome this ADR set out to
prevent. The rule's original phrasing ("that 404 before listing") measured the wrong thing: a
client-rendered not-found page never returns 404, so the dead link satisfied the rule as written.
Both the rule and the override are corrected here; the rule now turns on whether the URL resolves to
a live listing, not on its status code.

**Why the listing still does not exist — upstream, and not actionable here.** The submission
[community-operators#9070](https://github.com/k8s-operatorhub/community-operators/pull/9070)
("operator [N] [CI] kollect (0.18.0)", head `821cf2da`) has been open since 2026-08-18. Its
*Operator test* and *DCO test* workflow runs sit at `conclusion=action_required` — GitHub's
first-time-contributor "Approve and run" gate, which only a **k8s-operatorhub maintainer** can
clear. Everything on our side is green (`operator-ci`, `operator-automerge-enabled`, DCO), the
`authorized-changes` / `new-operator` / `allow-operator-recreate` labels are applied, and the PR
timeline shows no bot request, no review and no maintainer activity since 2026-08-19. Nothing is
being asked of us, and resubmitting does not clear an `action_required` run. The OpenShift sibling
[community-operators-prod#10889](https://github.com/redhat-openshift-ecosystem/community-operators-prod/pull/10889)
(0.18.0) is **merged**, so the OLM bundle is genuinely live in that catalog.

**What changed in the docs.** The badge and the prose keep naming OperatorHub/OLM — the OLM install
path is real — but point at the install page's *Discoverability on package hubs* section instead of
at the non-existent package page, and the badge is relabelled to the thing that is live (the OLM
bundle, package `kollect`, channel `stable`). `hack/test/dist_install_docs_test.sh` now asserts both
sides: the live destination must be present in `README.md`, and the `operatorhub.io` package URL must
be **absent** from `README.md` and `docs/getting-started/install.md`. When #9070 merges, re-point the
badge at the listing and flip that absence assertion back to a presence assertion. Artifact Hub is
untouched — that listing is live and its badge is asserted present, as before.

## Alternatives considered

See table. FBC rejected as disproportionate for solo maintenance. Artifact-Hub-only rejected
because OpenShift catalog adopters are in scope for this track.

### FBC posture — re-confirmed 2026-08-18

The upstream community-operators hosted pipeline on our open submissions
([community-operators#9070](https://github.com/k8s-operatorhub/community-operators/pull/9070),
[community-operators-prod#10889](https://github.com/redhat-openshift-ecosystem/community-operators-prod/pull/10889))
emits `check_using_fbc` as a **Warning**: *"File Based Catalog (FBC) is a new way to manage
operator metadata. This operator does not use FBC and it is recommended for new operators to
start directly with FBC."* Both submissions are green — the check recommends, it does not block.

We read the recommendation and kept **registry+v1 hand-templated bundles**; option C stays
rejected on its recorded grounds, not on new ones. In-repo FBC/`opm` is the only option that
loses on *operability / lean tooling* (2 against A's 4 and B's 5) and lands at the same weighted
total as doing nothing (35). The cost that decided it is ongoing, not one-off: an `opm`-rendered
catalog is a second generated artifact to keep, pin, and re-render every release, on top of the
CSV that `hack/test/dist_olm_bundle_test.sh` already gates for drift — disproportionate for a
solo-maintained project whose release engineering is deliberately lean
([ADR-0705](0705-release-supply-chain.md)).

This is a posture, not a permanent refusal. **Revisit when any of these becomes true:**

- upstream deprecates registry+v1 — `check_using_fbc` turns from a Warning into an error, or
  either catalog stops accepting non-FBC submissions;
- the bundle outgrows hand-templating — multiple channels, `skips`/`replaces` upgrade graphs, or
  per-OCP-version catalog pinning, none of which the single `stable` channel needs today;
- maintenance stops being solo, so the operability weighting that decided the table no longer
  dominates.

Adopting FBC reverses a recorded decision: it needs a superseding ADR, not an edit here.
