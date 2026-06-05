import type { Condition } from "@/api/k8s-status";

export type HealthLevel = "ready" | "degraded" | "syncing" | "unknown";

export const healthLabels: Record<HealthLevel, string> = {
  ready: "Ready",
  degraded: "Degraded",
  syncing: "Syncing",
  unknown: "Unknown",
};

export function isGenerationStale(
  generation?: number,
  observedGeneration?: number,
): boolean {
  return (
    generation !== undefined &&
    observedGeneration !== undefined &&
    observedGeneration < generation
  );
}

export function deriveHealthFromConditions(
  conditions?: Condition[],
  opts?: { generation?: number; observedGeneration?: number },
): HealthLevel {
  if (isGenerationStale(opts?.generation, opts?.observedGeneration)) {
    return "syncing";
  }

  if (!conditions?.length) {
    return "unknown";
  }

  if (conditions.some((c) => c.type === "Degraded" && c.status === "True")) {
    return "degraded";
  }

  if (
    conditions.some(
      (c) => (c.type === "Synced" || c.type === "Ready") && c.status === "False",
    )
  ) {
    return "degraded";
  }

  if (conditions.some((c) => c.type === "Synced" && c.status === "True")) {
    return "ready";
  }

  if (conditions.some((c) => c.type === "Ready" && c.status === "True")) {
    return "ready";
  }

  return "unknown";
}

export function deriveHealthFromExportStatus(status?: string): HealthLevel {
  if (status === "ok") {
    return "ready";
  }
  if (status === "degraded") {
    return "degraded";
  }
  return "unknown";
}
