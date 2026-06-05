import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";
import type { Condition } from "@/api/k8s-status";
import { fetchInventorySummary, type ExportStatus } from "@/api/inventory";
import { DetailDrawer } from "@/components/drawer/DetailDrawer";
import { buildSinkYamlSnippet } from "@/components/drawer/resourceYaml";
import { HealthBadge } from "@/components/status/HealthBadge";

type SelectedSink = ExportStatus;

function exportStatusToConditions(sink: ExportStatus): Condition[] {
  return [
    {
      type: "Ready",
      status: sink.status === "ok" ? "True" : "False",
      reason: sink.status,
      message: sink.message,
      lastTransitionTime: sink.lastExportTime,
    },
  ];
}

function sinkLabel(sink: ExportStatus): string {
  return sink.sinkNamespace ? `${sink.sinkNamespace}/${sink.sinkName}` : sink.sinkName;
}

export function SinksPage() {
  const [selected, setSelected] = useState<SelectedSink | null>(null);
  const returnFocusRef = useRef<HTMLButtonElement | null>(null);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["inventory", "summary", "sinks"],
    queryFn: () => fetchInventorySummary({ namespace: "team-a", limit: 1 }),
  });

  const sinks = data?.exportStatus ?? [];

  const yamlSnippet =
    selected &&
    buildSinkYamlSnippet({
      name: selected.sinkName,
      namespace: selected.sinkNamespace ?? "team-a",
      status: selected.status,
      message: selected.message,
    });

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-2xl font-semibold text-kollect-navy">Sinks</h1>
        <p className="text-sm text-slate-600">
          Export sink health from inventory summary exportStatus. Click a row for details.
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

      {!isLoading && !isError && sinks.length === 0 ? (
        <div className="rounded-lg border border-dashed border-slate-300 bg-white p-6 text-sm text-slate-600">
          No export sinks configured.
        </div>
      ) : null}

      {sinks.length > 0 ? (
        <div className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
          <table aria-label="Sinks" className="min-w-full text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
              <tr>
                <th scope="col" className="px-4 py-3 font-medium">
                  Sink
                </th>
                <th scope="col" className="px-4 py-3 font-medium">
                  Health
                </th>
                <th scope="col" className="px-4 py-3 font-medium">
                  Last export
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {sinks.map((sink) => (
                <tr
                  key={`${sink.sinkNamespace ?? ""}/${sink.sinkName}`}
                  className="hover:bg-slate-50"
                >
                  <td className="px-4 py-0">
                    <button
                      ref={
                        selected?.sinkName === sink.sinkName &&
                        selected.sinkNamespace === sink.sinkNamespace
                          ? returnFocusRef
                          : undefined
                      }
                      type="button"
                      className="w-full py-3 text-left font-medium text-kollect-navy hover:underline"
                      onClick={() => setSelected(sink)}
                    >
                      {sinkLabel(sink)}
                    </button>
                  </td>
                  <td className="px-4 py-3">
                    <HealthBadge exportStatus={sink.status} />
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-600">
                    {sink.lastExportTime ?? "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {selected && yamlSnippet ? (
        <DetailDrawer
          open={Boolean(selected)}
          onOpenChange={(open) => {
            if (!open) {
              setSelected(null);
            }
          }}
          returnFocusRef={returnFocusRef}
          title={selected.sinkName}
          subtitle={selected.sinkNamespace ?? "team-a"}
          conditions={exportStatusToConditions(selected)}
          yamlSnippet={yamlSnippet}
        />
      ) : null}
    </div>
  );
}
