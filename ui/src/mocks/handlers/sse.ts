import { http, HttpResponse } from "msw";
import { nextWatchSnapshot, getAllInventoryItems, buildInventorySummary } from "../fixtures";
import {
  filterInventoryItems,
  paginateItems,
  parseInventoryQuery,
} from "./utils";

function watchIntervalMs(url: URL): number {
  if (url.searchParams.get("mockWatch") === "burst") {
    return 50;
  }

  const fromEnv = import.meta.env.VITE_MOCK_WATCH_INTERVAL_MS;
  if (fromEnv) {
    const parsed = Number.parseInt(fromEnv, 10);
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed;
    }
  }

  return 5000;
}

function buildWatchPayload(request: Request, tick: number) {
  const url = new URL(request.url);
  const query = parseInventoryQuery(url);
  const filtered = filterInventoryItems(getAllInventoryItems(), query);
  const { page, limit, offset, total, hasMore } = paginateItems(
    filtered,
    query.limit,
    query.offset,
  );

  const base = nextWatchSnapshot(tick);
  const summary = buildInventorySummary(page.length > 0 ? page : base.items, {
    namespace: query.namespace ?? base.namespace,
    inventory: query.inventory ?? base.inventory,
    limit,
    offset,
    updatedAt: base.updatedAt,
    checksum: base.checksum,
  });

  summary.itemCount = summary.items.length;
  summary.pagination = { limit, offset, total, hasMore };
  return summary;
}

export const sseHandlers = [
  http.get("/v1alpha1/inventory/watch", ({ request }) => {
    const url = new URL(request.url);
    const intervalMs = watchIntervalMs(url);
    const encoder = new TextEncoder();
    let tick = 0;

    const stream = new ReadableStream({
      start(controller) {
        const send = () => {
          tick += 1;
          const payload = buildWatchPayload(request, tick);
          controller.enqueue(encoder.encode(`data: ${JSON.stringify(payload)}\n\n`));
        };

        send();
        const id = setInterval(send, intervalMs);
        request.signal.addEventListener("abort", () => {
          clearInterval(id);
          controller.close();
        });
      },
    });

    return new HttpResponse(stream, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
  }),
];
