import { AlertTriangle } from "lucide-react";
import type { ResourceStatus } from "@/api/k8s-status";
import { collectDegradedResources } from "@/lib/conditions";

type DegradedResourcesStripProps = {
  targets?: ResourceStatus[];
  inventories?: ResourceStatus[];
  loading?: boolean;
};

export function DegradedResourcesStrip({
  targets = [],
  inventories = [],
  loading = false,
}: DegradedResourcesStripProps) {
  if (loading) {
    return (
      <div
        className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm"
        role="status"
        aria-label="Loading degraded resources"
      >
        <div className="h-4 w-40 animate-pulse rounded bg-slate-200" />
        <div className="mt-3 space-y-2">
          <div className="h-10 animate-pulse rounded bg-slate-100" />
          <div className="h-10 animate-pulse rounded bg-slate-100" />
        </div>
      </div>
    );
  }

  const degraded = collectDegradedResources(targets, inventories);
  if (degraded.length === 0) {
    return null;
  }

  return (
    <section
      className="rounded-lg border border-amber-300 bg-amber-50 shadow-sm"
      role="region"
      aria-label="Degraded resources"
    >
      <div className="flex items-center gap-2 border-b border-amber-200 px-4 py-3">
        <AlertTriangle className="size-4 text-amber-700" aria-hidden="true" />
        <h2 className="text-sm font-semibold text-amber-950">
          {degraded.length} Degraded
        </h2>
      </div>
      <ul className="divide-y divide-amber-200">
        {degraded.map((row) => (
          <li
            key={`${row.kind}/${row.namespace}/${row.name}`}
            className="px-4 py-3 text-sm"
          >
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
              <span className="font-mono font-medium text-kollect-navy">
                {row.kind}/{row.name}
              </span>
              <span className="text-slate-600">{row.namespace}</span>
              <span className="rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-900">
                {row.reason}
              </span>
            </div>
            {row.message ? (
              <p className="mt-1 text-slate-700">{row.message}</p>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
}
