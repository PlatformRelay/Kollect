# Example: Hub mode

No **`KollectHub` CRD** — use Helm `mode: hub` ([ADR-0703](../adr/0703-platform-architecture-pivot.md)).

Spoke: `mode: spoke`. Hub: `hub.sinkRefs`, `hub.remoteClusters`, `hub.exportNamespace`.

Register spokes: `config/samples/kollect_v1alpha1_kollectremotecluster.yaml` (apply separately).

Transport default: `inprocess` ([ADR-0502](../adr/0502-lean-queue-transport.md)).
