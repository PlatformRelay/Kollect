import { describe, expect, it } from "vitest";
import type { ExportStatus } from "@/api/inventory";
import type { ResourceStatus } from "@/api/k8s-status";
import {
  collectDegradedResources,
  conditionSeverity,
  formatRelativeTime,
  isDegraded,
  primaryCondition,
  sortDegradedResources,
  summarizeExportStatus,
} from "./conditions";

describe("isDegraded", () => {
  it("returns true when Degraded condition is True", () => {
    expect(
      isDegraded([{ type: "Degraded", status: "True", reason: "PartialSync" }]),
    ).toBe(true);
  });

  it("returns true when Synced is False", () => {
    expect(
      isDegraded([{ type: "Synced", status: "False", reason: "ReconcileError" }]),
    ).toBe(true);
  });

  it("returns true when Ready is False", () => {
    expect(
      isDegraded([{ type: "Ready", status: "False", reason: "NotReady" }]),
    ).toBe(true);
  });

  it("returns true when observedGeneration lags generation", () => {
    expect(isDegraded([], { generation: 5, observedGeneration: 4 })).toBe(true);
  });

  it("returns false for healthy Synced target", () => {
    expect(
      isDegraded(
        [{ type: "Synced", status: "True", reason: "Collecting" }],
        { generation: 3, observedGeneration: 3 },
      ),
    ).toBe(false);
  });

  it("returns false for empty conditions without generation drift", () => {
    expect(isDegraded(undefined)).toBe(false);
  });
});

describe("conditionSeverity", () => {
  it("ranks Degraded above Synced=False", () => {
    const degraded = conditionSeverity([
      { type: "Degraded", status: "True" },
      { type: "Synced", status: "False" },
    ]);
    const syncing = conditionSeverity([{ type: "Synced", status: "False" }]);
    expect(degraded).toBeGreaterThan(syncing);
  });

  it("assigns non-zero severity for generation drift", () => {
    expect(conditionSeverity([], { generation: 2, observedGeneration: 1 })).toBeGreaterThan(0);
  });
});

describe("primaryCondition", () => {
  it("prefers Degraded over Synced=False", () => {
    const primary = primaryCondition([
      { type: "Synced", status: "False", reason: "ReconcileError", message: "sync msg" },
      { type: "Degraded", status: "True", reason: "PartialSync", message: "degraded msg" },
    ]);
    expect(primary?.type).toBe("Degraded");
    expect(primary?.reason).toBe("PartialSync");
  });
});

describe("sortDegradedResources", () => {
  it("sorts by severity desc, then namespace, then age", () => {
    const sorted = sortDegradedResources([
      {
        kind: "KollectTarget",
        name: "b",
        namespace: "team-b",
        reason: "A",
        message: "",
        severity: 2,
        lastTransitionTime: "2026-06-05T10:00:00Z",
      },
      {
        kind: "KollectTarget",
        name: "a",
        namespace: "team-a",
        reason: "B",
        message: "",
        severity: 3,
        lastTransitionTime: "2026-06-05T09:00:00Z",
      },
      {
        kind: "KollectTarget",
        name: "c",
        namespace: "team-a",
        reason: "C",
        message: "",
        severity: 3,
        lastTransitionTime: "2026-06-05T08:00:00Z",
      },
    ]);

    expect(sorted.map((r) => r.name)).toEqual(["c", "a", "b"]);
  });
});

describe("collectDegradedResources", () => {
  it("collects only degraded targets and inventories", () => {
    const targets: ResourceStatus[] = [
      {
        name: "healthy",
        namespace: "team-a",
        generation: 1,
        observedGeneration: 1,
        conditions: [{ type: "Synced", status: "True" }],
      },
      {
        name: "legacy-batch",
        namespace: "team-a",
        generation: 5,
        observedGeneration: 4,
        conditions: [
          { type: "Synced", status: "False", reason: "ReconcileError", message: "RBAC issue" },
          {
            type: "Degraded",
            status: "True",
            reason: "PartialSync",
            message: "2 of 5 GVKs failing",
            lastTransitionTime: "2026-06-05T09:15:00Z",
          },
        ],
      },
    ];
    const inventories: ResourceStatus[] = [
      {
        name: "team-inventory",
        namespace: "team-b",
        conditions: [{ type: "Synced", status: "True" }],
      },
    ];

    const rows = collectDegradedResources(targets, inventories);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      kind: "KollectTarget",
      name: "legacy-batch",
      namespace: "team-a",
      reason: "PartialSync",
      message: "2 of 5 GVKs failing",
    });
  });
});

describe("summarizeExportStatus", () => {
  it("rolls up sink counts and latest export time", () => {
    const exportStatus: ExportStatus[] = [
      { sinkName: "git", status: "ok", lastExportTime: "2026-06-05T11:55:00Z" },
      {
        sinkName: "s3",
        status: "degraded",
        lastExportTime: "2026-06-05T10:30:00Z",
        message: "push failed",
      },
      { sinkName: "prom", status: "unknown" },
    ];

    expect(summarizeExportStatus(exportStatus)).toEqual({
      lastExportTime: "2026-06-05T11:55:00Z",
      ok: 1,
      degraded: 1,
      unknown: 1,
      worst: "degraded",
    });
  });

  it("returns empty rollup when export status is missing", () => {
    expect(summarizeExportStatus(undefined)).toEqual({
      lastExportTime: undefined,
      ok: 0,
      degraded: 0,
      unknown: 0,
      worst: "unknown",
    });
  });
});

describe("formatRelativeTime", () => {
  it("formats minutes ago relative to a reference time", () => {
    const ref = new Date("2026-06-05T12:00:00Z");
    expect(formatRelativeTime("2026-06-05T11:55:00Z", ref)).toBe("5m ago");
  });
});
