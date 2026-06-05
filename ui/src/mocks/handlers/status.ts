import { http, HttpResponse } from "msw";
import { getInventoryStatus, getTargetStatus } from "../fixtures";
import { filterStatusItems } from "./utils";

export const statusHandlers = [
  http.get("/v1alpha1/status/targets", ({ request }) => {
    const url = new URL(request.url);
    const namespace = url.searchParams.get("namespace") ?? undefined;
    const data = getTargetStatus();
    return HttpResponse.json({
      ...data,
      items: filterStatusItems(data.items, namespace),
    });
  }),

  http.get("/v1alpha1/status/inventories", ({ request }) => {
    const url = new URL(request.url);
    const namespace = url.searchParams.get("namespace") ?? undefined;
    const data = getInventoryStatus();
    return HttpResponse.json({
      ...data,
      items: filterStatusItems(data.items, namespace),
    });
  }),
];
