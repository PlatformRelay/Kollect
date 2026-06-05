import { http, HttpResponse } from "msw";
import {
  buildInventorySummary,
  getAllInventoryItems,
} from "../fixtures";
import {
  filterInventoryItems,
  paginateItems,
  parseInventoryQuery,
} from "./utils";

function inventoryResponse(request: Request, pathNamespace?: string, pathInventory?: string) {
  const url = new URL(request.url);
  const query = parseInventoryQuery(url);

  if (pathNamespace) {
    query.namespace = pathNamespace;
  }
  if (pathInventory) {
    query.inventory = pathInventory;
  }

  const filtered = filterInventoryItems(getAllInventoryItems(), query);
  const { page, limit, offset, total, hasMore } = paginateItems(
    filtered,
    query.limit,
    query.offset,
  );

  const summary = buildInventorySummary(page, {
    namespace: query.namespace ?? pathNamespace,
    inventory: query.inventory ?? pathInventory,
    limit,
    offset,
  });

  summary.itemCount = page.length;
  summary.pagination = { limit, offset, total, hasMore };

  return HttpResponse.json(summary);
}

export const inventoryHandlers = [
  http.get("/v1alpha1/inventory", ({ request }) => inventoryResponse(request)),

  http.get("/v1alpha1/inventory/:namespace/:name", ({ request, params }) =>
    inventoryResponse(request, params.namespace as string, params.name as string),
  ),
];
