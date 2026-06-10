# Team-owned operator (minimal RBAC)

Platform teams run the **golden path**: one cluster-wide Kollect operator plus per-tenant
`KollectScope` objects ([ADR-0203](../adr/0203-namespaced-multi-tenancy.md)). Team-owned installs
are supported at **lower documentation priority** but must work with **minimal RBAC** beyond CRD
bootstrap (see [ADR-0203](../adr/0203-namespaced-multi-tenancy.md) RBAC expectations).

## Golden path vs team path

| Aspect | Golden path (platform) | Team path (this guide) |
| --- | --- | --- |
| Operator count | One cluster-wide release | One release per team namespace |
| Helm RBAC | `ClusterRole` / `ClusterRoleBinding` | `Role` / `RoleBinding` in install namespace |
| Watch scope | All namespaces (`watchNamespaces: []`) | Explicit list (`watchNamespaces: [team-a]`) |
| Tenancy policy | `KollectScope` per tenant namespace | Same — `KollectScope` in team namespace |
| CRD install | Cluster-scoped (standard) | Same — still requires cluster-level CRD apply |
| Cluster CRDs | `KollectClusterInventory`, `KollectClusterTarget`, etc. | **Not reconciled** — namespaced CRDs only |
| Validating webhooks | On by default | Usually **off** — `ValidatingWebhookConfiguration` is cluster-scoped |
| Overlap | N/A (single operator) | **Allowed** — multiple operators may watch the same GVK/namespace |

Overlapping watch scopes are **not prohibited**. Duplicate collection is an operational choice;
optional sink dedupe is a backstop only ([ADR-0305](../adr/0305-aggregation-dedupe.md),
[topology matrix — dedupe runbook](topology-matrix.md#multi-operator-sink-dedupe-runbook)).

## Install

### 1. CRDs (cluster admin, once per cluster)

CRDs are cluster-scoped. A platform admin applies them once; team installs reuse the same CRD set.

```bash
helm upgrade --install kollect-crds ./charts/kollect \
  --namespace kollect-system --create-namespace \
  --skip-crds=false
```

To install only CRDs without the operator Deployment, use your platform's CRD-only workflow or apply
`charts/kollect/crds/` with `kubectl`.

### 2. Team operator (namespace admin)

Use the chart profile [`values-minimal-rbac.yaml`](../../charts/kollect/values-minimal-rbac.yaml):

```bash
helm upgrade --install kollect-team ./charts/kollect \
  --namespace team-a --create-namespace \
  -f charts/kollect/values-minimal-rbac.yaml \
  --set watchNamespaces[0]=team-a
```

Set `watchNamespaces` to every namespace the team's informer cache should see (typically the install
namespace only). Non-empty `watchNamespaces` is required for this profile.

### 3. Workload collection RBAC (team)

The chart `Role` grants reconciler access to **Kollect CRDs and secrets in the install namespace
only**. It does **not** list/watch workload objects (Deployments, Services, etc.) cluster-wide.

Before `KollectTarget` can collect workloads, grant the operator ServiceAccount `get`, `list`, and
`watch` on the target GVKs in each scraped namespace. Kollect checks permissions via
`SelfSubjectAccessReview` before registering informers.

Example for `apps/v1` Deployments in `team-a` (adjust `subjects` to match your Helm release SA):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kollect-workload-reader
  namespace: team-a
rules:
  - apiGroups: [apps]
    resources: [deployments]
    verbs: [get, list, watch]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kollect-team-workload-reader
  namespace: team-a
subjects:
  - kind: ServiceAccount
    name: kollect-team
    namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kollect-workload-reader
```

Repeat or extend rules for each GVK referenced by team `KollectProfile` objects.

### 4. Team CRs

Apply namespaced `KollectScope`, `KollectProfile`, family sinks, `KollectTarget`, and
`KollectInventory` in the team namespace. Sample bundle:
[`config/samples/team-operator/`](../../config/samples/team-operator/).

## Reconciler RBAC verbs (tenant mode)

Helm `tenantMode: true` renders a namespaced `Role` with these rules:

| API group | Resources | Verbs | Purpose |
| --- | --- | --- | --- |
| `authorization.k8s.io` | `selfsubjectaccessreviews` | `create` | SAR pre-check before dynamic informers |
| `events.k8s.io` | `events` | `create`, `patch` | Status / warning events |
| `""` | `secrets` | `get`, `list`, `watch` | Sink credentials in **install namespace only** |
| `kollect.dev` | namespaced kinds (profiles, sinks, targets, inventories, scopes, connection tests) | full reconcile set | CR reconciliation |
| `kollect.dev` | `*/status`, `*/finalizers` | as generated | Status and finalizers |

Leader election uses a separate namespaced `Role` (`configmaps`, `leases`, core `events`) in the
install namespace.

**Not included** (by design — platform golden path only):

| Capability | Why omitted in tenant mode |
| --- | --- |
| Cluster-wide `secrets` / `namespaces` list | Blast radius — credentials stay in team namespace |
| `ClusterRole` on Kollect cluster CRDs | Team path uses namespaced inventory/target/sink only |
| `tokenreviews` / `subjectaccessreviews` | Inventory HTTP disabled in minimal profile |
| `ValidatingWebhookConfiguration` | Cluster-scoped; platform may run webhooks separately |

## Tradeoffs (honest)

- **Secrets:** Sink DSNs and Git credentials must live as Secrets in the team namespace. The operator
  cannot read Secrets in other namespaces.
- **Webhooks:** With `webhooks.enabled: false`, invalid CRs are caught at reconcile time, not admission.
  Platform teams may run a shared validating webhook if policy requires admission-time rejection.
- **Cluster rollups:** `KollectClusterInventory` and `KollectClusterTarget` require cluster-scoped
  reconciler RBAC — use the platform golden-path operator for federation.
- **Overlap:** A team operator and a platform operator may both watch `team-a`. Kollect does not block
  this; coordinate via ops policy or sink dedupe if duplicate rows are undesirable.

## See also

- [Deployment topology matrix](topology-matrix.md) — compare golden path, team path, hybrid, and overlap scenarios
- [ADR-0203: Namespaced multi-tenancy](../adr/0203-namespaced-multi-tenancy.md)
- [Helm values — per-team install](../operator-manual/helm-values.md#per-team-install-recommended-default)
- [Multi-tenant watch scope example](../examples/multi-tenant-watch-namespaces.md)
- [Chart `values-minimal-rbac.yaml`](../../charts/kollect/values-minimal-rbac.yaml)
