export type Item = {
  targetNamespace: string;
  targetName: string;
  namespace: string;
  name: string;
  group?: string;
  version: string;
  kind: string;
  uid: string;
  attributes: Record<string, unknown>;
};

export type Pagination = {
  limit: number;
  offset: number;
  total: number;
  hasMore: boolean;
};

export type ExportStatus = {
  sinkName: string;
  sinkNamespace?: string;
  status: string;
  lastExportTime?: string;
  message?: string;
};

export type InventorySummary = {
  schemaVersion: string;
  itemCount: number;
  namespace?: string;
  inventory?: string;
  items: Item[];
  updatedAt: string;
  pagination?: Pagination;
  exportStatus?: ExportStatus[];
  checksum?: string;
};

export type InventoryQuery = {
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

const baseUrl = import.meta.env.VITE_READ_API_URL ?? "";

function buildUrl(path: string, query?: Record<string, string | number | undefined>) {
  const url = new URL(path, baseUrl || window.location.origin);
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }
  }

  return url.toString();
}

export async function fetchInventorySummary(
  query: InventoryQuery = {},
): Promise<InventorySummary> {
  const response = await fetch(
    buildUrl("/v1alpha1/inventory", {
      namespace: query.namespace,
      inventory: query.inventory,
      target: query.target,
      group: query.group,
      version: query.version,
      kind: query.kind,
      name: query.name,
      limit: query.limit,
      offset: query.offset,
    }),
  );

  if (!response.ok) {
    throw new Error(`Read API ${response.status}: ${response.statusText}`);
  }

  return response.json() as Promise<InventorySummary>;
}
