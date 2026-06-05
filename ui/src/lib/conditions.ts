import type { ExportStatus } from "@/api/inventory";
import type { Condition, ResourceStatus } from "@/api/k8s-status";

export type ConditionInput = Pick<
  Condition,
  "type" | "status" | "reason" | "message" | "lastTransitionTime"
>;

export type DegradedResourceRow = {
  kind: "KollectTarget" | "KollectInventory";
  name: string;
  namespace: string;
  reason: string;
  message: string;
  lastTransitionTime?: string;
  severity: number;
};

export type ExportStatusRollup = {
  lastExportTime?: string;
  ok: number;
  degraded: number;
  unknown: number;
  worst: "ok" | "degraded" | "unknown";
};

const SEVERITY_DEGRADED = 3;
const SEVERITY_UNSYNCED = 2;
const SEVERITY_GENERATION_DRIFT = 1;

function hasGenerationDrift(generation?: number, observedGeneration?: number): boolean {
  return (
    generation !== undefined &&
    observedGeneration !== undefined &&
    observedGeneration < generation
  );
}

export function isDegraded(
  conditions?: ConditionInput[],
  opts?: { generation?: number; observedGeneration?: number },
): boolean {
  if (hasGenerationDrift(opts?.generation, opts?.observedGeneration)) {
    return true;
  }

  if (!conditions?.length) {
    return false;
  }

  if (conditions.some((c) => c.type === "Degraded" && c.status === "True")) {
    return true;
  }

  if (conditions.some((c) => c.type === "Synced" && c.status === "False")) {
    return true;
  }

  if (conditions.some((c) => c.type === "Ready" && c.status === "False")) {
    return true;
  }

  return false;
}

export function conditionSeverity(
  conditions?: ConditionInput[],
  opts?: { generation?: number; observedGeneration?: number },
): number {
  if (!conditions?.length && !hasGenerationDrift(opts?.generation, opts?.observedGeneration)) {
    return 0;
  }

  if (conditions?.some((c) => c.type === "Degraded" && c.status === "True")) {
    return SEVERITY_DEGRADED;
  }

  if (
    conditions?.some(
      (c) => (c.type === "Synced" || c.type === "Ready") && c.status === "False",
    )
  ) {
    return SEVERITY_UNSYNCED;
  }

  if (hasGenerationDrift(opts?.generation, opts?.observedGeneration)) {
    return SEVERITY_GENERATION_DRIFT;
  }

  return 0;
}

export function primaryCondition(
  conditions?: ConditionInput[],
): ConditionInput | undefined {
  if (!conditions?.length) {
    return undefined;
  }

  const degraded = conditions.find((c) => c.type === "Degraded" && c.status === "True");
  if (degraded) {
    return degraded;
  }

  const synced = conditions.find((c) => c.type === "Synced" && c.status === "False");
  if (synced) {
    return synced;
  }

  const ready = conditions.find((c) => c.type === "Ready" && c.status === "False");
  if (ready) {
    return ready;
  }

  return undefined;
}

export function sortDegradedResources(rows: DegradedResourceRow[]): DegradedResourceRow[] {
  return [...rows].sort((a, b) => {
    if (b.severity !== a.severity) {
      return b.severity - a.severity;
    }

    const ns = a.namespace.localeCompare(b.namespace);
    if (ns !== 0) {
      return ns;
    }

    const ta = a.lastTransitionTime ?? "";
    const tb = b.lastTransitionTime ?? "";
    return ta.localeCompare(tb);
  });
}

function resourceToDegradedRow(
  kind: DegradedResourceRow["kind"],
  resource: ResourceStatus,
): DegradedResourceRow | null {
  const opts = { generation: resource.generation, observedGeneration: resource.observedGeneration };

  if (!isDegraded(resource.conditions, opts)) {
    return null;
  }

  const primary = primaryCondition(resource.conditions);
  const drift = hasGenerationDrift(resource.generation, resource.observedGeneration);

  return {
    kind,
    name: resource.name,
    namespace: resource.namespace,
    reason: primary?.reason ?? (drift ? "GenerationDrift" : "Unknown"),
    message:
      primary?.message ??
      (drift ? "Status has not caught up to the latest spec generation" : ""),
    lastTransitionTime: primary?.lastTransitionTime,
    severity: conditionSeverity(resource.conditions, opts),
  };
}

export function collectDegradedResources(
  targets: ResourceStatus[],
  inventories: ResourceStatus[],
): DegradedResourceRow[] {
  const rows: DegradedResourceRow[] = [];

  for (const target of targets) {
    const row = resourceToDegradedRow("KollectTarget", target);
    if (row) {
      rows.push(row);
    }
  }

  for (const inventory of inventories) {
    const row = resourceToDegradedRow("KollectInventory", inventory);
    if (row) {
      rows.push(row);
    }
  }

  return sortDegradedResources(rows);
}

export function summarizeExportStatus(
  exportStatus: ExportStatus[] | undefined,
): ExportStatusRollup {
  if (!exportStatus?.length) {
    return { lastExportTime: undefined, ok: 0, degraded: 0, unknown: 0, worst: "unknown" };
  }

  let lastExportTime: string | undefined;
  let ok = 0;
  let degraded = 0;
  let unknown = 0;

  for (const sink of exportStatus) {
    if (sink.status === "ok") {
      ok += 1;
    } else if (sink.status === "degraded") {
      degraded += 1;
    } else {
      unknown += 1;
    }

    if (
      sink.lastExportTime &&
      (!lastExportTime || sink.lastExportTime > lastExportTime)
    ) {
      lastExportTime = sink.lastExportTime;
    }
  }

  const worst: ExportStatusRollup["worst"] =
    degraded > 0 ? "degraded" : unknown > 0 ? "unknown" : "ok";

  return { lastExportTime, ok, degraded, unknown, worst };
}

export function formatRelativeTime(iso: string, reference = new Date()): string {
  const diffMs = reference.getTime() - Date.parse(iso);
  if (Number.isNaN(diffMs) || diffMs < 0) {
    return "just now";
  }

  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }

  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h ago`;
  }

  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}
