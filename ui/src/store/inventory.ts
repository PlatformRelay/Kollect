import { create } from "zustand";
import type { InventoryQuery } from "@/api/inventory";

export type InventoryFilters = {
  namespace?: string;
  inventory?: string;
  kind?: string;
  target?: string;
  search?: string;
  limit?: number;
  offset?: number;
};

export type ColumnVisibility = {
  kind: boolean;
  namespace: boolean;
  name: boolean;
  target: boolean;
  group: boolean;
  version: boolean;
};

export const DEFAULT_COLUMN_VISIBILITY: ColumnVisibility = {
  kind: true,
  namespace: true,
  name: true,
  target: true,
  group: false,
  version: false,
};

const COLUMN_VISIBILITY_KEY = "kollect-ui:inventory-columns";
const DEFAULT_LIMIT = 500;
const DEFAULT_OFFSET = 0;

type InventoryStoreState = {
  columnVisibility: ColumnVisibility;
  setColumnVisibility: (patch: Partial<ColumnVisibility>) => void;
  resetColumnVisibility: () => void;
  hydrateColumnVisibility: () => void;
};

export function filtersFromSearch(raw: Record<string, unknown>): InventoryFilters {
  const filters: InventoryFilters = {};

  if (typeof raw.namespace === "string" && raw.namespace) {
    filters.namespace = raw.namespace;
  }
  if (typeof raw.inventory === "string" && raw.inventory) {
    filters.inventory = raw.inventory;
  }
  if (typeof raw.kind === "string" && raw.kind) {
    filters.kind = raw.kind;
  }
  if (typeof raw.target === "string" && raw.target) {
    filters.target = raw.target;
  }
  if (typeof raw.search === "string" && raw.search) {
    filters.search = raw.search;
  }
  if (raw.limit !== undefined && raw.limit !== "") {
    const limit = Number(raw.limit);
    if (Number.isFinite(limit) && limit > 0) {
      filters.limit = limit;
    }
  }
  if (raw.offset !== undefined && raw.offset !== "") {
    const offset = Number(raw.offset);
    if (Number.isFinite(offset) && offset >= 0) {
      filters.offset = offset;
    }
  }

  return filters;
}

export function searchFromFilters(filters: InventoryFilters): Record<string, string | number> {
  const search: Record<string, string | number> = {};

  if (filters.namespace) {
    search.namespace = filters.namespace;
  }
  if (filters.inventory) {
    search.inventory = filters.inventory;
  }
  if (filters.kind) {
    search.kind = filters.kind;
  }
  if (filters.target) {
    search.target = filters.target;
  }
  if (filters.search) {
    search.search = filters.search;
  }
  if (filters.limit !== undefined) {
    search.limit = filters.limit;
  }
  if (filters.offset !== undefined) {
    search.offset = filters.offset;
  }

  return search;
}

export function filtersToQuery(filters: InventoryFilters): InventoryQuery {
  return {
    namespace: filters.namespace,
    inventory: filters.inventory,
    kind: filters.kind,
    target: filters.target,
    name: filters.search,
    limit: filters.limit ?? DEFAULT_LIMIT,
    offset: filters.offset ?? DEFAULT_OFFSET,
  };
}

export function inventoryQueryKey(filters: InventoryFilters = {}) {
  const active: InventoryFilters = {};
  if (filters.namespace) active.namespace = filters.namespace;
  if (filters.inventory) active.inventory = filters.inventory;
  if (filters.kind) active.kind = filters.kind;
  if (filters.target) active.target = filters.target;
  if (filters.search) active.search = filters.search;
  if (filters.limit !== undefined) active.limit = filters.limit;
  if (filters.offset !== undefined) active.offset = filters.offset;

  return Object.keys(active).length > 0
    ? (["inventory", "list", active] as const)
    : (["inventory", "list"] as const);
}

function readColumnVisibility(): ColumnVisibility {
  if (typeof window === "undefined") {
    return DEFAULT_COLUMN_VISIBILITY;
  }

  try {
    const raw = window.localStorage.getItem(COLUMN_VISIBILITY_KEY);
    if (!raw) {
      return DEFAULT_COLUMN_VISIBILITY;
    }
    const parsed = JSON.parse(raw) as Partial<ColumnVisibility>;
    return { ...DEFAULT_COLUMN_VISIBILITY, ...parsed };
  } catch {
    return DEFAULT_COLUMN_VISIBILITY;
  }
}

function writeColumnVisibility(visibility: ColumnVisibility) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(COLUMN_VISIBILITY_KEY, JSON.stringify(visibility));
}

export const useInventoryStore = create<InventoryStoreState>((set, get) => ({
  columnVisibility: DEFAULT_COLUMN_VISIBILITY,
  setColumnVisibility: (patch) => {
    const next = { ...get().columnVisibility, ...patch };
    writeColumnVisibility(next);
    set({ columnVisibility: next });
  },
  resetColumnVisibility: () => {
    writeColumnVisibility(DEFAULT_COLUMN_VISIBILITY);
    set({ columnVisibility: DEFAULT_COLUMN_VISIBILITY });
  },
  hydrateColumnVisibility: () => {
    set({ columnVisibility: readColumnVisibility() });
  },
}));
