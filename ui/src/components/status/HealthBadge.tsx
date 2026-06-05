import { AlertTriangle, CheckCircle2, HelpCircle, RefreshCw } from "lucide-react";
import type { Condition } from "@/api/k8s-status";
import {
  deriveHealthFromConditions,
  deriveHealthFromExportStatus,
  healthLabels,
  type HealthLevel,
} from "./health";

type HealthBadgeProps = {
  level?: HealthLevel;
  conditions?: Condition[];
  exportStatus?: string;
  generation?: number;
  observedGeneration?: number;
};

const toneClasses: Record<HealthLevel, string> = {
  ready: "border-kollect-teal/30 bg-kollect-teal/10 text-teal-900",
  degraded: "border-red-200 bg-red-50 text-red-900",
  syncing: "border-amber-200 bg-amber-50 text-amber-900",
  unknown: "border-slate-200 bg-slate-100 text-slate-600",
};

const icons: Record<HealthLevel, typeof CheckCircle2> = {
  ready: CheckCircle2,
  degraded: AlertTriangle,
  syncing: RefreshCw,
  unknown: HelpCircle,
};

export function HealthBadge({
  level,
  conditions,
  exportStatus,
  generation,
  observedGeneration,
}: HealthBadgeProps) {
  const resolved =
    level ??
    (exportStatus !== undefined
      ? deriveHealthFromExportStatus(exportStatus)
      : deriveHealthFromConditions(conditions, { generation, observedGeneration }));

  const Icon = icons[resolved];
  const label = healthLabels[resolved];

  return (
    <span
      role="status"
      aria-label={label}
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${toneClasses[resolved]}`}
    >
      <Icon className="size-3.5 shrink-0" aria-hidden="true" />
      {label}
    </span>
  );
}
