import { http, HttpResponse } from "msw";
import type { InventorySummary } from "@/api/inventory";
import type { StatusListResponse } from "@/api/k8s-status";

const mockInventory: InventorySummary = {
  schemaVersion: "kollect.dev/v1alpha1",
  itemCount: 2,
  namespace: "team-a",
  updatedAt: "2026-06-05T12:00:00Z",
  items: [
    {
      targetNamespace: "team-a",
      targetName: "deploys",
      namespace: "apps",
      name: "web",
      group: "apps",
      version: "v1",
      kind: "Deployment",
      uid: "uid-1",
      attributes: { replicas: 2 },
    },
    {
      targetNamespace: "team-a",
      targetName: "deploys",
      namespace: "apps",
      name: "api",
      group: "apps",
      version: "v1",
      kind: "Deployment",
      uid: "uid-2",
      attributes: { replicas: 1 },
    },
  ],
  pagination: { limit: 50, offset: 0, total: 2, hasMore: false },
};

const mockTargets: StatusListResponse = {
  schemaVersion: "kollect.dev/v1alpha1",
  items: [
    {
      name: "deploys",
      namespace: "team-a",
      conditions: [{ type: "Synced", status: "True", message: "Collecting" }],
    },
  ],
};

const mockInventories: StatusListResponse = {
  schemaVersion: "kollect.dev/v1alpha1",
  items: [
    {
      name: "team-inventory",
      namespace: "team-a",
      itemCount: 2,
      conditions: [{ type: "Synced", status: "True" }],
    },
  ],
};

export const handlers = [
  http.get("/v1alpha1/inventory", () => HttpResponse.json(mockInventory)),
  http.get("/v1alpha1/status/targets", () => HttpResponse.json(mockTargets)),
  http.get("/v1alpha1/status/inventories", () => HttpResponse.json(mockInventories)),
];
