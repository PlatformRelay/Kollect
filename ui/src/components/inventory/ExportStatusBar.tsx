import type { ExportStatus } from "@/api/inventory";

type ExportPhase = "ok" | "degraded" | "unknown";

const PHASE_STYLES: Record<ExportPhase, string> = {
  ok: "bg-emerald-100 text-emerald-800",
  degraded: "bg-amber-100 text-amber-900",
  unknown: "bg-slate-100 text-slate-700",
};

function deriveExportPhase(status: ExportStatus["status"]): ExportPhase {
  if (status === "ok") {
    return "ok";
  }
  if (status === "degraded") {
    return "degraded";
  }
  return "unknown";
}

function formatRelativeTime(iso?: string): string {
  if (!iso) {
    return "never";
  }

  const then = Date.parse(iso);
  if (!Number.isFinite(then)) {
    return iso;
  }

  const deltaSec = Math.round((Date.now() - then) / 1000);
  if (deltaSec < 60) {
    return `${deltaSec}s ago`;
  }
  const deltaMin = Math.round(deltaSec / 60);
  if (deltaMin < 60) {
    return `${deltaMin}m ago`;
  }
  const deltaHr = Math.round(deltaMin / 60);
  return `${deltaHr}h ago`;
}

type ExportStatusBarProps = {
  statuses?: ExportStatus[];
};

export function ExportStatusBar({ statuses }: ExportStatusBarProps) {
  if (!statuses?.length) {
    return null;
  }

  return (
    <div
      role="region"
      aria-label="Export status"
      className="flex flex-wrap gap-2 rounded-lg border border-slate-200 bg-white p-3 shadow-sm"
    >
      {statuses.map((entry) => {
        const phase = deriveExportPhase(entry.status);
        const sinkLabel = entry.sinkNamespace
          ? `${entry.sinkNamespace}/${entry.sinkName}`
          : entry.sinkName;

        return (
          <div
            key={`${entry.sinkNamespace ?? ""}/${entry.sinkName}`}
            className="flex items-center gap-2 rounded-md border border-slate-100 bg-slate-50 px-3 py-1.5 text-xs"
            title={entry.message}
          >
            <span className="font-medium text-kollect-navy">{sinkLabel}</span>
            <span
              role="status"
              className={`inline-flex rounded px-2 py-0.5 font-medium ${PHASE_STYLES[phase]}`}
            >
              {entry.status}
            </span>
            <span className="text-slate-500">{formatRelativeTime(entry.lastExportTime)}</span>
          </div>
        );
      })}
    </div>
  );
}
