# Example: Hub mode

!!! info "Optional fleet tier"
    Hub mode is for multi-cluster aggregation when shared-sink fan-in is insufficient (Git fan-in,
    network isolation, credential centralization). Single-cluster installs do not need a hub.

No **`KollectHub` CRD** — use Helm `mode: hub` ([ADR-0703](../adr/0703-platform-architecture-pivot.md)).

Spoke: `mode: spoke`. Hub: `hub.sinkRefs`, `hub.remoteClusters`, `hub.exportNamespace`.

Register spokes: `config/samples/kollect_v1alpha1_kollectremotecluster.yaml` (apply separately).

!!! warning "Pre-beta hub transport"
    Hub ingest and spoke push paths are still maturing. Validate against your environment before
    production fleet rollout; see [ADR-0501](../adr/0501-multi-cluster-sync-rfc.md).

Transport default: `inprocess` ([ADR-0502](../adr/0502-lean-queue-transport.md)).
