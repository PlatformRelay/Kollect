# RFC: Separate CRDs per sink family (Option C)

**Status:** Superseded → [ADR-0414](../adr/0414-sink-family-crds.md) · **Author:** @konih · **Created:** 2026-06-06

> **Implemented as ADR-0414 (clean break, pre-GA).** The unified `KollectSink` CRD was removed; inventories
> reference family sinks via `snapshotSinkRefs`, `databaseSinkRefs`, and `eventSinkRefs`. See the ADR for
> the accepted design, migration notes, and CRD inventory.

## Historical context

This RFC explored Option C — three family CRDs aligned with [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)
(snapshot, relational, event) instead of a single flat `KollectSink`. The exploration informed ADR-0414;
the detailed proposal, alternatives, and dual-support migration window below are **retained for archaeology
only** and do not describe the shipped API.

---

<details>
<summary>Original RFC body (archived)</summary>

## Summary

As kollect adds export backends (BigQuery, Azure Blob, HTTP/webhook, Parquet modes, and future
cluster-scoped sinks), the single **`KollectSink`** CRD with a flat `spec.type` enum and optional
sibling blocks may become **ergonomically and operationally costly** — even though CRD byte size is not
a blocker today. This RFC proposed **Option C**: split sink configuration into **three family CRDs**
aligned with ADR-0401 roles (snapshot, relational, event), with typed inventory references per family.

*(Remaining original sections omitted in this archive — see git history before ADR-0414 merge.)*

</details>
