import { describe, expect, it } from "vitest";
import {
  deriveHealthFromConditions,
  deriveHealthFromExportStatus,
  isGenerationStale,
} from "./health";

describe("health helpers", () => {
  it("marks generation drift as stale", () => {
    expect(isGenerationStale(5, 4)).toBe(true);
    expect(isGenerationStale(3, 3)).toBe(false);
  });

  it("derives degraded health from Degraded or Synced=False conditions", () => {
    expect(
      deriveHealthFromConditions([
        { type: "Synced", status: "False", reason: "ReconcileError" },
        { type: "Degraded", status: "True", reason: "PartialSync" },
      ]),
    ).toBe("degraded");

    expect(
      deriveHealthFromConditions([{ type: "Synced", status: "True", reason: "Collecting" }]),
    ).toBe("ready");
  });

  it("derives export status health for sinks", () => {
    expect(deriveHealthFromExportStatus("ok")).toBe("ready");
    expect(deriveHealthFromExportStatus("degraded")).toBe("degraded");
    expect(deriveHealthFromExportStatus("unknown")).toBe("unknown");
  });
});
