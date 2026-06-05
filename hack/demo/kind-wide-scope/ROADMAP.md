# Wide-scope kind demo — roadmap

> Early-adopter checklist for [`hack/demo/kind-wide-scope/`](./). Maintainer design notes live in
> local `agent-context/DEMO-KIND-WIDE-SCOPE-PROPOSAL.md` (not in git).

**Status:** P0 shipped (2026-06-05) — venue pitch + lab personas, fast churn, UI reveal path.

---

## Quick start

```sh
bash hack/demo/kind-wide-scope/demo.sh --check
export GITHUB_TOKEN="$(gh auth token)"   # skip for local persona
bash hack/demo/kind-wide-scope/demo.sh --churn --reveal
DEMO_PERSONA=local bash hack/demo/kind-wide-scope/demo.sh --reveal
```

---

## Prerequisites (minimum versions)

| Tool | Minimum | Notes |
| --- | --- | --- |
| kind | 0.20+ | Cluster name `kollect-dev` |
| kubectl | 1.28+ | Matches repo k8s pin |
| helm | 3.12+ | Platform operators + operator chart |
| kustomize | 5.0+ | Bundled with kubectl or standalone |
| task | 3.x | `task kind-dev-up` |
| docker | 20+ | kind node image load |
| gh | 2.x | Git export proof (optional for `local` persona) |
| go | 1.22+ | Auto-install Charm Gum when missing |
| gum | latest | Optional pre-install; guided UX |

Verify: `bash hack/demo/kind-wide-scope/demo.sh --check` or `task demo-check`.

---

## Personas

| Persona | Overlay | Platform operators | Git token |
| --- | --- | --- | --- |
| **full** (default) | `overlays/full` | Trivy + cert-manager + ESO | Required |
| **security** | `overlays/security` | Yes | Required |
| **platform** | `overlays/platform` | Skipped | Required |
| **local** | `overlays/local` | Yes | Not required |

Set via Gum menu, `DEMO_PERSONA=…`, or `--skip-platform` (platform only).

---

## Churn presets

| Preset | Flag / env | Cycle | Use |
| --- | --- | --- | --- |
| **fast** | `--churn` / `CHURN_PRESET=fast` | ~2–4 min | Default venue mode |
| **present** | `--churn=present` | ~12 min | Narrated talking track |
| **burst** | `--churn=burst` | ~60 s | Recorded demo / asciinema |

Schedule: [`churn/steps.yaml`](churn/steps.yaml) — edit steps without bash surgery.

| Step (fast order) | Inventory delta |
| --- | --- |
| Image bump frontend (early) | `image` + new Trivy report |
| Scale api-gateway | `replicas` |
| Label patch backend | `labels` |
| ConfigMap feature-flags | `keyCount` / data |
| Delete catalog-sync | Row removed |
| Create billing-api | Row added |
| Suspend weekly-report | `suspend: true` |
| Recreate storefront Service | `clusterIP` refresh |

Export debounce: `exportMinInterval: 15s` on `team-inventory` — fast preset gaps beat debounce.

---

## P0 checklist (shipped)

- [x] DWS-01 `--check` / `task demo-check`
- [x] DWS-02 Minimum versions table (this file)
- [x] DWS-03 `DEMO_AUTO_YES=1` CI-safe
- [x] DWS-04 `--reuse-cluster` / `--fresh`
- [x] DWS-10 Persona menu + `DEMO_PERSONA`
- [x] DWS-11 Flags: `--churn`, `--reveal`, `--skip-platform`, `--help`
- [x] DWS-12 Optional step-0 apiserver contrast
- [x] DWS-13 Local persona (no GitHub token)
- [x] DWS-14 Trivy wait / honest copy when VR=0
- [x] DWS-15 No double confirm on `--churn`
- [x] DWS-16–19 Kustomize overlays (`full`, `security`, `platform`, `local`)
- [x] DWS-20 UI via `charts/kollect/ci/demo-values.yaml`
- [x] DWS-21 `--reveal` port-forward + URLs
- [x] DWS-30–33 Declarative churn (`churn/steps.yaml`, `run.sh`, `manifests/`)
- [x] DWS-40 `lib/reveal.sh`
- [x] DWS-42 Git commit link via `gh api` (when authenticated)
- [x] DWS-43 End card links
- [x] DWS-60 Public ROADMAP (this file)
- [x] DWS-61 README venue story
- [x] DWS-62 Examples index cross-link

---

## P1 stretch (deferred)

- [ ] DWS-50 `KollectConnectionTest` sample beat
- [ ] DWS-51 `--show-scope-deny` micro-step
- [ ] DWS-52 Profile custom metrics sidebar
- [ ] DWS-53 `exportMinInterval` narration in churn table (partial — see above)
- [ ] DWS-54 `logs.sh --follow-export-only`
- [ ] DWS-55 Recorded demo profile + asciinema note
- [ ] DWS-41 `--present` tmux 3-pane layout

---

## Hardening (deferred)

- [ ] DWS-H01 Pin Helm chart versions in `install-platform.sh`
- [ ] DWS-H02 Vendor Gum / pure-bash fallback
- [ ] DWS-H03 `task demo-bundle` container
- [ ] DWS-H04 Offline Git sink (gitea in kind)
- [ ] DWS-H05 CI smoke: `DEMO_AUTO_YES=1 demo.sh --check && kustomize build …`
- [ ] DWS-H06 Shellcheck gate on full demo tree in CI

---

## Validate locally

```sh
kustomize build hack/demo/kind-wide-scope/ >/dev/null
kustomize build hack/demo/kind-wide-scope/overlays/full >/dev/null
shellcheck hack/demo/kind-wide-scope/*.sh hack/demo/kind-wide-scope/lib/*.sh hack/demo/kind-wide-scope/churn/*.sh
task scrub
```

---

## See also

- [README.md](./README.md) — venue pitch + runbook
- [samples/](./samples/) — annotated Kollect CRs
- [Kind local lab](../../docs/examples/kind-local-lab.md)
- [UI local development](../../docs/examples/ui-local-development.md)
- [Examples index](../../docs/examples/README.md)
