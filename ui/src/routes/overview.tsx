import { useQuery } from "@tanstack/react-query";
import { fetchInventorySummary } from "@/api/inventory";
import { fetchInventoryStatus, fetchTargetStatus } from "@/api/k8s-status";

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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-kollect-navy">Overview</h1>
        <p className="mt-1 text-sm text-slate-600">
          Read-only observability shell — inventory rows from the Read API, CR health from status endpoints.
        </p>
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Items (sample)" value={inventory.data?.itemCount ?? "—"} loading={inventory.isLoading} />
        <StatCard label="Targets" value={targets.data?.items.length ?? "—"} loading={targets.isLoading} />
        <StatCard label="Inventories" value={inventories.data?.items.length ?? "—"} loading={inventories.isLoading} />
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  loading,
}: {
  label: string;
  value: string | number;
  loading: boolean;
}) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-2 text-3xl font-semibold text-kollect-navy">
        {loading ? "…" : value}
      </p>
    </div>
  );
}
