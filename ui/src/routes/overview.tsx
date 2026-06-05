import { useQuery } from "@tanstack/react-query";
import { fetchInventorySummary } from "@/api/inventory";
import { fetchInventoryStatus, fetchTargetStatus } from "@/api/k8s-status";
import { DegradedResourcesStrip } from "@/components/overview/DegradedResourcesStrip";
import { formatRelativeTime, summarizeExportStatus } from "@/lib/conditions";

export function OverviewPage() {
  const inventory = useQuery({
    queryKey: ["inventory", "overview"],
    queryFn: () => fetchInventorySummary({ limit: 5 }),
  });
  const targets = useQuery({
    queryKey: ["status", "targets"],
    queryFn: () => fetchTargetStatus(),
  });
  const inventories = useQuery({
    queryKey: ["status", "inventories"],
    queryFn: () => fetchInventoryStatus(),
  });

  const exportRollup = summarizeExportStatus(inventory.data?.exportStatus);
  const itemTotal = inventory.data?.pagination?.total ?? inventory.data?.itemCount;
  const statusLoading = targets.isLoading || inventories.isLoading;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-kollect-navy">Overview</h1>
        <p className="mt-1 text-sm text-slate-600">
          Read-only observability shell — inventory rows from the Read API, CR health from status endpoints.
        </p>
      </div>

      <DegradedResourcesStrip
        targets={targets.data?.items}
        inventories={inventories.data?.items}
        loading={statusLoading}
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Item count"
          value={itemTotal ?? "—"}
          detail={
            inventory.data?.pagination
              ? `${inventory.data.items.length} shown in sample`
              : undefined
          }
          loading={inventory.isLoading}
        />
        <StatCard
          label="Last export"
          value={
            exportRollup.lastExportTime
              ? formatRelativeTime(exportRollup.lastExportTime)
              : "—"
          }
          detail={exportStatusDetail(exportRollup)}
          loading={inventory.isLoading}
          tone={exportRollup.worst === "degraded" ? "warning" : undefined}
        />
        <StatCard
          label="Export health"
          value={`${exportRollup.ok} ok`}
          detail={exportHealthDetail(exportRollup)}
          loading={inventory.isLoading}
          tone={exportRollup.degraded > 0 ? "warning" : exportRollup.unknown > 0 ? "muted" : "ok"}
        />
        <StatCard
          label="Targets"
          value={targets.data?.items.length ?? "—"}
          loading={targets.isLoading}
        />
      </div>
    </div>
  );
}

function exportStatusDetail(rollup: ReturnType<typeof summarizeExportStatus>): string | undefined {
  if (!rollup.lastExportTime) {
    return rollup.degraded > 0 || rollup.unknown > 0 ? "No successful export yet" : undefined;
  }
  return rollup.worst === "degraded" ? "At least one sink is degraded" : undefined;
}

function exportHealthDetail(rollup: ReturnType<typeof summarizeExportStatus>): string | undefined {
  const parts: string[] = [];
  if (rollup.degraded > 0) {
    parts.push(`${rollup.degraded} degraded sink${rollup.degraded === 1 ? "" : "s"}`);
  }
  if (rollup.unknown > 0) {
    parts.push(`${rollup.unknown} unknown`);
  }
  return parts.length > 0 ? parts.join(" · ") : "All sinks reporting ok";
}

function StatCard({
  label,
  value,
  detail,
  loading,
  tone,
}: {
  label: string;
  value: string | number;
  detail?: string;
  loading: boolean;
  tone?: "ok" | "warning" | "muted";
}) {
  const valueTone =
    tone === "warning"
      ? "text-amber-800"
      : tone === "ok"
        ? "text-emerald-700"
        : "text-kollect-navy";

  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className={`mt-2 text-3xl font-semibold ${valueTone}`}>
        {loading ? "…" : value}
      </p>
      {detail && !loading ? (
        <p className="mt-1 text-xs text-slate-600">{detail}</p>
      ) : null}
    </div>
  );
}
