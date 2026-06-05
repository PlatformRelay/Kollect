import { describe, expect, it } from "vitest";

describe("mock inventory handlers", () => {
  it("returns paginated inventory with hasMore and total", async () => {
    const response = await fetch("/v1alpha1/inventory?limit=10&offset=0");
    expect(response.status).toBe(200);

    const body = (await response.json()) as {
      itemCount: number;
      pagination: { limit: number; offset: number; total: number; hasMore: boolean };
      exportStatus?: unknown[];
    };

    expect(body.itemCount).toBe(10);
    expect(body.pagination).toEqual({
      limit: 10,
      offset: 0,
      total: 120,
      hasMore: true,
    });
    expect(body.exportStatus?.length).toBeGreaterThan(0);
  });

  it("filters inventory by kind and target", async () => {
    const response = await fetch(
      "/v1alpha1/inventory?namespace=team-a&target=deploys&kind=Deployment",
    );
    const body = (await response.json()) as { items: Array<{ kind: string; targetName: string }> };

    expect(body.items.length).toBeGreaterThan(0);
    expect(body.items.every((i) => i.kind === "Deployment" && i.targetName === "deploys")).toBe(
      true,
    );
  });

  it("returns inventory by namespace and name path", async () => {
    const response = await fetch("/v1alpha1/inventory/team-a/team-inventory?limit=5");
    expect(response.status).toBe(200);

    const body = (await response.json()) as {
      namespace?: string;
      inventory?: string;
      pagination: { total: number };
    };

    expect(body.namespace).toBe("team-a");
    expect(body.inventory).toBe("team-inventory");
    expect(body.pagination.total).toBe(120);
  });
});

describe("mock status handlers", () => {
  it("filters targets by namespace", async () => {
    const response = await fetch("/v1alpha1/status/targets?namespace=team-a");
    const body = (await response.json()) as { items: Array<{ namespace: string }> };

    expect(body.items.length).toBe(3);
    expect(body.items.every((i) => i.namespace === "team-a")).toBe(true);
  });

  it("includes degraded target conditions", async () => {
    const response = await fetch("/v1alpha1/status/targets?namespace=team-a");
    const body = (await response.json()) as {
      items: Array<{ name: string; conditions?: Array<{ type: string; status: string }> }>;
    };

    const legacy = body.items.find((i) => i.name === "legacy-batch");
    expect(legacy?.conditions?.some((c) => c.type === "Degraded" && c.status === "True")).toBe(
      true,
    );
  });
});

describe("mock SSE watch handler", () => {
  it("streams at least one inventory snapshot event", async () => {
    const controller = new AbortController();
    const response = await fetch("/v1alpha1/inventory/watch?mockWatch=burst", {
      signal: controller.signal,
    });

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toContain("text/event-stream");

    const reader = response.body!.getReader();
    const decoder = new TextDecoder();
    let chunk = "";

    for (let i = 0; i < 20 && !chunk.includes("data:"); i++) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }
      chunk += decoder.decode(value, { stream: true });
    }

    controller.abort();
    await reader.cancel();

    expect(chunk).toMatch(/data: \{"schemaVersion":"kollect\.dev\/v1alpha1"/);
  });
});
