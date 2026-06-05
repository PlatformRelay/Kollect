import type { InventoryFilters } from "@/store/inventory";

type InventoryFilterBarProps = {
  filters: InventoryFilters;
  onChange: (patch: Partial<InventoryFilters>) => void;
};

export function InventoryFilterBar({ filters, onChange }: InventoryFilterBarProps) {
  return (
    <form
      aria-label="Inventory filters"
      className="grid gap-3 rounded-lg border border-slate-200 bg-white p-4 shadow-sm md:grid-cols-2 lg:grid-cols-4"
      onSubmit={(event) => event.preventDefault()}
    >
      <label className="space-y-1 text-sm">
        <span className="font-medium text-slate-700">Namespace</span>
        <input
          className="w-full rounded-md border border-slate-300 px-3 py-2"
          name="namespace"
          value={filters.namespace ?? ""}
          onChange={(event) => onChange({ namespace: event.target.value || undefined })}
          placeholder="team-a"
        />
      </label>

      <label className="space-y-1 text-sm">
        <span className="font-medium text-slate-700">Kind</span>
        <input
          className="w-full rounded-md border border-slate-300 px-3 py-2"
          name="kind"
          value={filters.kind ?? ""}
          onChange={(event) => onChange({ kind: event.target.value || undefined })}
          placeholder="Deployment"
        />
      </label>

      <label className="space-y-1 text-sm">
        <span className="font-medium text-slate-700">Target</span>
        <input
          className="w-full rounded-md border border-slate-300 px-3 py-2"
          name="target"
          value={filters.target ?? ""}
          onChange={(event) => onChange({ target: event.target.value || undefined })}
          placeholder="deploys"
        />
      </label>

      <label className="space-y-1 text-sm">
        <span className="font-medium text-slate-700">Search name</span>
        <input
          className="w-full rounded-md border border-slate-300 px-3 py-2"
          name="search"
          value={filters.search ?? ""}
          onChange={(event) => onChange({ search: event.target.value || undefined })}
          placeholder="api"
        />
      </label>
    </form>
  );
}
