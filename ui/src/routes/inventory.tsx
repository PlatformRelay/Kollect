import { useQuery } from "@tanstack/react-query";
import { fetchInventorySummary } from "@/api/inventory";

export function InventoryPage() {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["inventory", "list"],
    queryFn: () => fetchInventorySummary({ limit: 50 }),
  });

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-2xl font-semibold text-kollect-navy">Inventory</h1>
        <p className="text-sm text-slate-600">Collected resource rows from the Read API.</p>
      </header>
      {isLoading && <p className="text-sm text-slate-500">Loading inventory…</p>}
      {isError && (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800" role="alert">
          {(error as Error).message}
        </p>
      )}
      {data && (
        <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white shadow-sm">
          <table className="min-w-full text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
              <tr>
                <th className="px-4 py-2">Kind</th>
                <th className="px-4 py-2">Namespace</th>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">Target</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((item) => (
                <tr key={item.uid} className="border-b border-slate-100 last:border-0">
                  <td className="px-4 py-2 font-mono text-xs">{item.kind}</td>
                  <td className="px-4 py-2">{item.namespace}</td>
                  <td className="px-4 py-2">{item.name}</td>
                  <td className="px-4 py-2 text-slate-600">
                    {item.targetNamespace}/{item.targetName}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="border-t border-slate-200 px-4 py-2 text-xs text-slate-500">
            schemaVersion {data.schemaVersion} · {data.itemCount} row(s)
          </p>
        </div>
      )}
    </div>
  );
}
