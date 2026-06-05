import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { fetchTargetStatus, type Condition, type ResourceStatus } from "@/api/k8s-status";
import { DetailDrawer } from "@/components/drawer/DetailDrawer";
import { buildResourceYamlSnippet } from "@/components/drawer/resourceYaml";
import { HealthBadge } from "@/components/status/HealthBadge";

function formatConditions(conditions?: Condition[]): string {
  if (!conditions?.length) {
    return "No conditions";
  }

  return conditions.map((c) => `${c.type}=${c.status}`).join(" · ");
}

export function TargetsPage() {
  const [selected, setSelected] = useState<ResourceStatus | null>(null);
  const returnFocusRef = useRef<HTMLButtonElement | null>(null);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["status", "targets"],
    queryFn: () => fetchTargetStatus(),
  });

  const yamlSnippet =
    selected &&
    buildResourceYamlSnippet({
      apiVersion: "kollect.dev/v1alpha1",
      kind: "KollectTarget",
      name: selected.name,
      namespace: selected.namespace,
      generation: selected.generation,
    });

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-2xl font-semibold text-kollect-navy">Targets</h1>
        <p className="text-sm text-slate-600">
          KollectTarget CR conditions from the Read API status proxy. Click a row for details.
        </p>
      </header>

      {isLoading && <p className="text-sm text-slate-500">Loading…</p>}
      {isError && (
        <p
          className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800"
          role="alert"
        >
          {(error as Error).message}
        </p>
      )}

      {data && (
        <div className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
          <table aria-label="Targets" className="min-w-full text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
              <tr>
                <th scope="col" className="px-4 py-3 font-medium">
                  Target
                </th>
                <th scope="col" className="px-4 py-3 font-medium">
                  Health
                </th>
                <th scope="col" className="px-4 py-3 font-medium">
                  Conditions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {data.items.map((item) => (
                <tr key={`${item.namespace}/${item.name}`} className="hover:bg-slate-50">
                  <td className="px-4 py-0">
                    <button
                      ref={selected?.name === item.name && selected.namespace === item.namespace ? returnFocusRef : undefined}
                      type="button"
                      className="w-full py-3 text-left font-medium text-kollect-navy hover:underline"
                      onClick={() => setSelected(item)}
                    >
                      {item.namespace}/{item.name}
                    </button>
                  </td>
                  <td className="px-4 py-3">
                    <HealthBadge
                      conditions={item.conditions}
                      generation={item.generation}
                      observedGeneration={item.observedGeneration}
                    />
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-600">
                    {formatConditions(item.conditions)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selected && yamlSnippet ? (
        <DetailDrawer
          open={Boolean(selected)}
          onOpenChange={(open) => {
            if (!open) {
              setSelected(null);
            }
          }}
          returnFocusRef={returnFocusRef}
          title={selected.name}
          subtitle={selected.namespace}
          conditions={selected.conditions}
          generation={selected.generation}
          observedGeneration={selected.observedGeneration}
          yamlSnippet={yamlSnippet}
        />
      ) : null}
    </div>
  );
}
