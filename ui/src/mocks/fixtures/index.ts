import type { ExportStatus, InventorySummary, Item } from "@/api/inventory";
import type { StatusListResponse } from "@/api/k8s-status";
import teamAInventory from "./inventory-team-a.json";
import exportStatusMixed from "./export-status-mixed.json";
import targetsDegraded from "./targets-degraded.json";
import inventoriesStatus from "./inventories-status.json";

const SCHEMA_VERSION = "kollect.dev/v1alpha1";
const BASE_UPDATED_AT = "2026-06-05T12:00:00Z";

type TeamAFixture = {
  namespace: string;
  inventory: string;
  updatedAt: string;
  checksum?: string;
  items: Item[];
};

const teamA = teamAInventory as TeamAFixture;
const exportStatuses = exportStatusMixed as ExportStatus[];
const allTargets = targetsDegraded as StatusListResponse;
const allInventories = inventoriesStatus as StatusListResponse;

/** Deterministic 120-row catalog for pagination tests (team-a deploys target). */
function buildLargeInventoryItems(count: number): Item[] {
  const items: Item[] = [...teamA.items];
  const kinds = ["Deployment", "ReplicaSet", "Pod", "Service", "ConfigMap"] as const;
  const groups = ["apps", "", "batch"] as const;

  for (let i = items.length; i < count; i++) {
    const kind = kinds[i % kinds.length]!;
    const group = groups[i % groups.length]!;
    items.push({
      targetNamespace: "team-a",
      targetName: "deploys",
      namespace: "apps",
      name: `workload-${String(i).padStart(4, "0")}`,
      group,
      version: "v1",
      kind,
      uid: `uid-large-${i}`,
      attributes: { index: i, replicas: (i % 5) + 1 },
    });
  }

  return items;
}

export const largeInventoryItems = buildLargeInventoryItems(120);

export function getAllInventoryItems(): Item[] {
  return largeInventoryItems;
}

export function getExportStatuses(): ExportStatus[] {
  return exportStatuses.map((s) => ({ ...s }));
}

export function getTargetStatus(): StatusListResponse {
  return {
    schemaVersion: allTargets.schemaVersion,
    items: allTargets.items.map((item) => ({
      ...item,
      conditions: item.conditions?.map((c) => ({ ...c })),
    })),
  };
}

export function getInventoryStatus(): StatusListResponse {
  return {
    schemaVersion: allInventories.schemaVersion,
    items: allInventories.items.map((item) => ({
      ...item,
      conditions: item.conditions?.map((c) => ({ ...c })),
    })),
  };
}

export function buildInventorySummary(
  items: Item[],
  options: {
    namespace?: string;
    inventory?: string;
    limit?: number;
    offset?: number;
    includeExportStatus?: boolean;
    updatedAt?: string;
    checksum?: string;
  } = {},
): InventorySummary {
  const limit = options.limit ?? 500;
  const offset = options.offset ?? 0;
  const total = items.length;
  const page = items.slice(offset, offset + limit);

  const summary: InventorySummary = {
    schemaVersion: SCHEMA_VERSION,
    itemCount: page.length,
    namespace: options.namespace ?? teamA.namespace,
    inventory: options.inventory ?? teamA.inventory,
    updatedAt: options.updatedAt ?? BASE_UPDATED_AT,
    items: page,
    pagination: {
      limit,
      offset,
      total,
      hasMore: offset + page.length < total,
    },
  };

  if (options.checksum) {
    summary.checksum = options.checksum;
  } else if (teamA.checksum) {
    summary.checksum = teamA.checksum;
  }

  if (options.includeExportStatus !== false) {
    summary.exportStatus = getExportStatuses();
  }

  return summary;
}

export function nextWatchSnapshot(tick: number): InventorySummary {
  const items = getAllInventoryItems();
  const bumped = new Date(Date.parse(BASE_UPDATED_AT) + tick * 1000).toISOString();
  const summary = buildInventorySummary(items.slice(0, 4), {
    updatedAt: bumped,
    checksum: `sha256:watch-tick-${tick}`,
  });
  summary.itemCount = summary.items.length;
  return summary;
}
