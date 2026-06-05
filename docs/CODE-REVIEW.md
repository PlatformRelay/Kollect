# Code review — 2026-06-05

Prioritized findings from security, scalability, and architecture-gap review.
**Fixed in this session** are marked ✅.

## P0 — production blockers

| ID | Finding | Status |
| --- | --- | --- |
| P0-1 | Hub HTTP ingest skipped cluster ACL (`ReceiveReport(..., nil)`) | ✅ Wired `AllowedClusters` from `KOLLECT_REMOTE_CLUSTERS` |
| P0-2 | Hub ingest auth does not bind token identity to cluster header | Open — needs `KollectRemoteCluster` owner lookup |
| P0-3 | Empty `KOLLECT_REMOTE_CLUSTERS` allows any cluster | Open — defer fail-closed until hub chart sets allowlist by default |

## P1 — security / correctness / scale

| ID | Finding | Status |
| --- | --- | --- |
| P1-1 | Hub ingest SAR missing hub namespace on `kollectremoteclusters` | ✅ `KOLLECT_PLATFORM_NAMESPACE` on SAR |
| P1-2 | Inventory HTTP SAR not namespace-scoped | Open |
| P1-3 | Inventory index endpoint missing `list` SAR | Open |
| P1-4 | Failed exports still recorded for debounce | ✅ `recordExport` only after all sinks succeed |
| P1-5 | Hub ingest has no TokenReview/SAR cache | Open |
| P1-6 | Hub HTTP ingest plain HTTP (no TLS) | Open — document mandatory termination |
| P1-7 | Secondary watch Profile → Targets missing | ✅ `mapProfileToTargets` watch |
| P1-8 | Target watch enqueues all inventories in namespace | By design — inventory aggregates all namespace targets |
| P1-9 | No `KollectSink` validating webhook | Open |
| P1-10 | CEL `cel:` prefix not required at admission | ✅ `ValidateAttributePath` rejects bare `object.*` |
| P1-11 | JSONPath filter validation Phase 1 warn-only | ✅ `ProfileWarnings` on `[?(` paths |
| P1-12 | `AccessChecker` SAR cache never expires | Open |

## P2 — structure / tech debt

| ID | Finding | Status |
| --- | --- | --- |
| P2-1 | `KollectHub` dead controller + CRD remnants | Open — reject webhook kept; controller unregistered |
| P2-2 | Duplicate `bearerToken` in inventory/hub auth | Open |
| P2-3 | Engine `dispatch()` O(targets) per informer event | Open — index by GVR |
| P2-4 | Store single `RWMutex` + full namespace snapshots | Open |
| P2-5 | Debounce state in-memory only (restart burst) | Accept for MVP |
| P2-6 | `resolveCAPEM` defaults secret namespace to `default` | Open |
| P2-7 | `docs/ROADMAP.md` stale on `exportMinInterval` | Open |
| P2-8 | Hub ingest body limit 8 MiB vs ADR 512 KiB inline | Open |
| P2-9 | Inventory HTTP path param unused for auth/filtering | Open |

## Architecture gaps (PLATFORM-DECISIONS)

| Item | Status |
| --- | --- |
| Hub ingest SAR `create` on `kollectremoteclusters` in hub namespace | ✅ Namespace + verb wired |
| JSONPath filter Phase 1 warn-only | ✅ |
| Namespaced `sinkRefs` | ✅ Already implemented |
| `exportMinInterval` per Inventory (30s default) | ✅ Already implemented |
| Secondary watch Sink → Inventories | ✅ Already implemented |
| Secondary watch Profile → Targets | ✅ Fixed this session |

## What is in good shape

- Inventory HTTP TokenReview + SAR with 30s cache
- Profile webhook Secret.data guard
- Shared informer per GVK
- Scope hard degrade
- TLS 1.2 minimum on git/transport sinks
- ClusterTarget `namespaceSelector` required
