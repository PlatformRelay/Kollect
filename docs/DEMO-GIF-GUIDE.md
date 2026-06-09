<!-- markdownlint-disable MD013 -->

# Demo GIF creation guide

Contributor-runnable playbook for recording the **hero story** demos with
[VHS](https://github.com/charmbracelet/vhs). Two parallel tracks:

| Variant | Length | Primary artifact | Best for |
| --- | --- | --- | --- |
| **A — Git-only** | ≤60s | `docs/assets/demo/hero-git-only.gif` (+ `.mp4`) | README teaser, docs index |
| **B — Git + Postgres** | ~2–3 min | `docs/assets/demo/hero-git-postgres.mp4` (GIF optional) | QUICKSTART deep dive, examples |

> **Hero sentence:** *Your cluster, in Git, diffable.* Declare GVK + CEL in CRDs; when the cluster
> changes, inventory commits change. `git log` is your audit trail; `git diff` is your drift report.

The harness uses an **in-kind Forgejo** Git server — no GitHub tokens, fully reproducible offline.

---

## Variant A — Git-only (README hero GIF)

### 1. Prerequisites (~5 min install)

| Tool | Purpose | Install |
| --- | --- | --- |
| Docker | kind nodes, image build | [docker.com](https://docs.docker.com/get-docker/) |
| [kind](https://kind.sigs.k8s.io/) | Local Kubernetes | `go install sigs.k8s.io/kind@latest` |
| kubectl | Cluster admin | Kubernetes docs |
| [Task](https://taskfile.dev/) | Repo task runner | `go install github.com/go-task/task/v3/cmd/task@latest` |
| git | Clone inventory repo | OS package manager |
| curl | Forgejo bootstrap | OS package manager |
| [VHS](https://github.com/charmbracelet/vhs) | Deterministic terminal recording | `go install github.com/charmbracelet/vhs@latest` |
| bat (optional) | Syntax-highlighted YAML in tape | `cargo install bat` or OS package |

Verify:

```sh
docker info >/dev/null && kind version && kubectl version --client && task --version && git --version
```

!!! note "Expected output"
    All commands exit 0. `kind version` reports `0.32.x` or newer; `task --version` reports `3.x`.

### 2. Environment setup (~10–15 min first run)

From the repo root:

```sh
task demo-hero-up
```

This runs `hack/demo/hero/up.sh` and:

1. Creates kind cluster **`kollect-hero`** (`hack/demo/hero/cluster.yaml`)
2. Builds and Helm-installs kollect (`charts/kollect/ci/dev-values.yaml`)
3. Deploys **Forgejo** in-cluster (`hack/demo/hero/manifests/forgejo.yaml`)
4. Bootstraps admin user, `inventory-demo` repo, and push token (`bootstrap-forgejo.sh`)
5. Applies the **shop** workload (`manifests/demo-workloads.yaml` — `shop:v2.0.0` in `demo` ns)
6. Applies the golden **Git-only** sample (`config/samples/demo/git-only/`)
7. Waits for `KollectInventory/demo-inventory` **Ready** and first Git export
8. Clones the inventory repo to **`/tmp/kollect-hero-inventory`** (port-forward on `localhost:13000`)

!!! note "Expected tail output"
    ```text
    [hero] Hero demo ready.
    [hero]   Inventory clone: /tmp/kollect-hero-inventory
    [hero]   Forgejo UI:      http://127.0.0.1:13000/
    [hero]   State file:      /tmp/kollect-hero-state.env
    ```

Tear down when finished:

```sh
task demo-hero-down
```

### 3. Golden sample (Git-only)

| Path | Kind | Role |
| --- | --- | --- |
| `config/samples/demo/git-only/profile.yaml` | `KollectProfile` | Extract Deployment `image` + `replicas` |
| `config/samples/demo/git-only/target.yaml` | `KollectTarget` | Select `shop` Deployments in `demo` ns |
| `config/samples/demo/git-only/sink.yaml` | `KollectSnapshotSink` | Push YAML to in-cluster Forgejo |
| `config/samples/demo/git-only/inventory.yaml` | `KollectInventory` | Debounce + export every 15s |

Single-file view for the tape (scene 3): `hack/demo/hero/kollect-demo.yaml`.

Apply manually (if not using `task demo-hero-up`):

```sh
kubectl apply -k config/samples/demo/git-only/
```

Git export layout: `namespaces/{sourceNamespace}/{kind}/{sourceName}.yaml` (YAML attributes per resource).

### 4. Pre-flight checks before recording (~1 min)

```sh
bash hack/demo/hero/preflight.sh
```

Confirms:

- `KollectInventory/demo-inventory` condition **Ready**
- `KollectSnapshotSink/hero-git-sink` **ConnectionVerified**
- Forgejo reachable on `localhost:13000`
- At least one exported file under `/tmp/kollect-hero-inventory/`

!!! note "Expected output"
    ```text
    [hero] Pre-flight OK — safe to record.
    ```

### 5. VHS tape (storyboard)

Checked-in tape: `hack/demo/hero/demo-git-only.tape`

| # | Scene | Command | Screen time |
| --- | --- | --- | --- |
| 1 | Title | `# kollect — your cluster, in Git, diffable` | 2s |
| 2 | Bootstrap | `task demo-hero-up` | 8s |
| 3 | Config = CRs | `bat hack/demo/hero/kollect-demo.yaml` | 8s |
| 4 | Ready | `kubectl get kinv -A` | 5s |
| 5 | First export | `git pull` + `ls` + `bat` exported YAML | 10s |
| 6 | Drift | `kubectl set image deploy/shop api=shop:v2.1.0 -n demo` | 4s |
| 7 | **Money shot** | `git log -p -1` (image `v2.0.0` → `v2.1.0`) | 12s |
| 8 | Close | audit-trail tagline + docs URL | 4s |

### 6. Recording commands (~2–5 min)

Install VHS if needed:

```sh
go install github.com/charmbracelet/vhs@latest
```

Record (cluster must already be up — tape assumes `demo-hero-up` completed):

```sh
task demo-hero-record-git-only
```

Equivalent:

```sh
bash hack/demo/hero/record.sh demo-git-only.tape
```

### 7. Post-processing and artifacts

| Artifact | Path | Budget |
| --- | --- | --- |
| GIF (README) | `docs/assets/demo/hero-git-only.gif` | **< 3 MB** |
| MP4 (docs embed) | `docs/assets/demo/hero-git-only.mp4` | no hard cap |

If the GIF exceeds 3 MB:

- Re-run VHS with `Set Width 1000` in the tape, or
- Lower framerate in post (`ffmpeg -i hero-git-only.mp4 -vf fps=10 ...`), or
- Split scenes 1–5 and 6–8 into two GIFs.

Regenerate locally — **do not commit multi-MB binaries** unless under budget.

### 8. Embed instructions

**README** (under intro, above “Why Kollect”):

```markdown
![Kollect demo: Deployment image change appears as a Git diff](docs/assets/demo/hero-git-only.gif)
*Your cluster, in Git, diffable — a Deployment image bump becomes an auditable commit.*
```

**docs/index.md** hero block — same GIF + alt text describing the diff moment.

**docs/QUICKSTART.md** — link to this guide in the Demo section; use MP4 for long-form:

```html
<video autoplay loop muted playsinline width="100%">
  <source src="../assets/demo/hero-git-postgres.mp4" type="video/mp4">
</video>
```

### 9. Troubleshooting (Variant A)

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `ConnectionVerified=False` on git sink | Missing `hero-git-credentials` Secret | Re-run `task demo-hero-up` or `bootstrap-forgejo.sh` |
| Git push auth errors in manager logs | Stale token | `task demo-hero-down && task demo-hero-up` |
| No export after drift | Debounce / `exportMinInterval` | Wait 20–30s; check `kubectl describe kinv demo-inventory` |
| Forgejo not ready | PVC pending on single-node kind | `kubectl get pvc -n forgejo`; delete cluster and retry |
| `git pull` empty | Port-forward stopped | `bash hack/demo/hero/preflight.sh` (restarts PF) |
| GIF > 3 MB | Default VHS resolution | Shrink width/fps (§7) |
| `bat: command not found` | Optional tool | `apt install bat` or change tape to `cat` |

---

## Variant B — Git + Postgres (long-form companion)

Same harness, extended with disposable Postgres (`config/samples/dev/postgres.yaml`) and
`config/samples/demo/git-postgres/`.

### 1. Prerequisites

Variant A prerequisites **plus** nothing extra — Postgres runs in-cluster.

### 2. Environment setup (~12–18 min first run)

```sh
task demo-hero-up-postgres
```

Additionally:

- Deploys `config/samples/dev/postgres.yaml` (emptyDir, dev-only credentials)
- Creates `inventory-postgres-dsn` Secret in `kollect-system`
- Applies `KollectDatabaseSink/hero-postgres-sink` with `provisioning.mode: ensure`
- Wires both sinks on `KollectInventory/demo-inventory`

!!! note "Expected output"
    ```text
    kollectdatabasesink/hero-postgres-sink condition met
    kollectsnapshotsink/hero-git-sink condition met
    kollectinventory/demo-inventory condition met
    ```

### 3. Golden sample (Git + Postgres)

Inherits Git-only resources from `config/samples/demo/git-only/` plus:

| Path | Kind | Role |
| --- | --- | --- |
| `config/samples/demo/git-postgres/databasesink.yaml` | `KollectDatabaseSink` | Postgres `inventory_items` table (`mode: ensure`) |
| `config/samples/demo/git-postgres/inventory.yaml` | `KollectInventory` | Parallel `snapshotSinkRefs` + `databaseSinkRefs` |

### 4. Pre-flight

```sh
bash hack/demo/hero/preflight.sh
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/hero-postgres-sink -n default --timeout=30s
```

### 5. VHS tape (extended storyboard)

Tape: `hack/demo/hero/demo-git-postgres.tape`

Adds after the Git money shot:

```sh
kubectl exec -n kollect-system deploy/postgres -- psql -U kollect -d inventory -c \
  "SELECT payload->'attributes'->>'image' AS image, COUNT(*) FROM public.inventory_items GROUP BY 1 ORDER BY 1;"
```

Optional scene: `kubectl get kollectinventory demo-inventory -o yaml` showing **both** sink ref lists
(parallel multi-sink is shipped — one inventory, multiple family sinks).

### 6. Recording

```sh
task demo-hero-record-git-postgres
```

### 7. Artifacts

| Artifact | Path | Notes |
| --- | --- | --- |
| MP4 (primary) | `docs/assets/demo/hero-git-postgres.mp4` | Embed in QUICKSTART / examples |
| GIF (teaser) | `docs/assets/demo/hero-git-postgres.gif` | Optional; MP4 preferred for 2–3 min |

### 8. When to use which variant

| Surface | Use |
| --- | --- |
| README first screen, GitHub social preview | Variant A GIF |
| docs/index.md hero | Variant A GIF |
| QUICKSTART “see it happen”, examples hub | Variant B MP4 + link to A |
| Conference / README “full story” link | Variant B MP4 on docs site |

### 9. Troubleshooting (Variant B)

All Variant A rows **plus**:

| Symptom | Fix |
| --- | --- |
| `ConnectionVerified=False` on Postgres sink | Apply `config/samples/dev/postgres.yaml` + DSN Secret (see QUICKSTART) |
| Empty `inventory_items` | Wait for first export after both sinks verified; `kubectl describe kinv demo-inventory` |
| psql query returns 0 rows | Confirm drift export ran; re-trigger with `kubectl annotate kinv demo-inventory kollect.dev/force-export=true` if needed |

---

## Maintainer notes

- **Recording polish** (pacing at 1×) is a maintainer step — tapes include `Hide`/`Sleep` jump-cuts for image pulls and debounce waits.
- **CI gate (optional later):** nightly `vhs` run against `task demo-hero-up` → `preflight.sh`.
- **Drift-PR GitHub Action** recipe is deferred — see RECOMMENDATIONS §2.3.
- If you cannot record in CI/agent env, the **tape + this guide + harness** are the deliverable; run
  `task demo-hero-record-git-only` locally to populate `docs/assets/demo/`.

## Related

- [QUICKSTART.md](QUICKSTART.md) · [examples/deployment-inventory.md](examples/deployment-inventory.md)
- Golden samples: `config/samples/demo/`
- Harness scripts: `hack/demo/hero/`
