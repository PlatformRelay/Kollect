import { inventoryHandlers } from "./handlers/inventory";
import { statusHandlers } from "./handlers/status";
import { sseHandlers } from "./handlers/sse";

export const handlers = [...inventoryHandlers, ...statusHandlers, ...sseHandlers];

export { inventoryHandlers, statusHandlers, sseHandlers };
