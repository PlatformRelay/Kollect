import { useQuery } from "@tanstack/react-query";
import { fetchTargetStatus } from "@/api/k8s-status";

export function TargetsPage() {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["status", "targets"],
    queryFn: () => fetchTargetStatus(),
  });

  return (
    <ResourceStatusPage
      title="Targets"
      description="KollectTarget CR conditions (Read API status proxy)."
      data={data}
      isLoading={isLoading}
      isError={isError}
      error={error}
    />
  );
}

type ResourceStatusPageProps = {
  title: string;
  description: string;
  data?: { schemaVersion: string; items: Array<{ name: string; namespace: string; conditions?: Array<{ type: string; status: string }> }> };
  isLoading: boolean;
  isError: boolean;
  error: unknown;
};

function ResourceStatusPage({
  title,
  description,
  data,
  isLoading,
  isError,
  error,
}: ResourceStatusPageProps) {
  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-2xl font-semibold text-kollect-navy">{title}</h1>
        <p className="text-sm text-slate-600">{description}</p>
      </header>
      {isLoading && <p className="text-sm text-slate-500">Loading…</p>}
      {isError && (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800" role="alert">
          {(error as Error).message}
        </p>
      )}
      {data && (
        <ul className="divide-y divide-slate-200 rounded-lg border border-slate-200 bg-white shadow-sm">
          {data.items.map((item) => (
            <li key={`${item.namespace}/${item.name}`} className="px-4 py-3">
              <p className="font-medium text-kollect-navy">
                {item.namespace}/{item.name}
              </p>
              <p className="mt-1 text-xs text-slate-500">
                {item.conditions?.map((c) => `${c.type}=${c.status}`).join(" · ") || "No conditions"}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
