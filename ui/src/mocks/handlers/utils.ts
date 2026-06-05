import type { Item } from "@/api/inventory";

export type InventoryQueryParams = {
  namespace?: string;
  inventory?: string;
  target?: string;
  group?: string;
  version?: string;
  kind?: string;
  name?: string;
  limit?: number;
  offset?: number;
};

const DEFAULT_LIMIT = 500;

export function parseInventoryQuery(url: URL): InventoryQueryParams {
  const intParam = (key: string): number | undefined => {
    const raw = url.searchParams.get(key);
    if (raw === null || raw === "") {
      return undefined;
    }
    const parsed = Number.parseInt(raw, 10);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
  };

  const strParam = (key: string): string | undefined => {
    const raw = url.searchParams.get(key);
    return raw === null || raw === "" ? undefined : raw;
  };

  return {
    namespace: strParam("namespace"),
    inventory: strParam("inventory"),
    target: strParam("target"),
    group: strParam("group"),
    version: strParam("version"),
    kind: strParam("kind"),
    name: strParam("name"),
    limit: intParam("limit"),
    offset: intParam("offset"),
  };
}

export function filterInventoryItems(items: Item[], query: InventoryQueryParams): Item[] {
  return items.filter((item) => {
    if (query.namespace && item.targetNamespace !== query.namespace && item.namespace !== query.namespace) {
      return false;
    }
    if (query.target && item.targetName !== query.target) {
      return false;
    }
    if (query.group !== undefined && (item.group ?? "") !== query.group) {
      return false;
    }
    if (query.version && item.version !== query.version) {
      return false;
    }
    if (query.kind && item.kind !== query.kind) {
      return false;
    }
    if (query.name && item.name !== query.name) {
      return false;
    }
    return true;
  });
}

export function paginateItems(
  items: Item[],
  limit = DEFAULT_LIMIT,
  offset = 0,
): { page: Item[]; limit: number; offset: number; total: number; hasMore: boolean } {
  const safeLimit = limit > 0 ? limit : DEFAULT_LIMIT;
  const safeOffset = offset >= 0 ? offset : 0;
  const total = items.length;
  const page = items.slice(safeOffset, safeOffset + safeLimit);

  return {
    page,
    limit: safeLimit,
    offset: safeOffset,
    total,
    hasMore: safeOffset + page.length < total,
  };
}

export function filterStatusItems<T extends { namespace: string }>(
  items: T[],
  namespace?: string,
): T[] {
  if (!namespace) {
    return items;
  }
  return items.filter((item) => item.namespace === namespace);
}
