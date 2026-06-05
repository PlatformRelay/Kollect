import { useQuery } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useMemo } from "react";
import { fetchInventorySummary, type Item } from "@/api/inventory";
import { DetailDrawer } from "@/components/drawer/DetailDrawer";
import { buildInventoryItemYamlSnippet } from "@/components/drawer/resourceYaml";
import { ExportStatusBar } from "@/components/inventory/ExportStatusBar";
import { InventoryFilterBar } from "@/components/inventory/InventoryFilterBar";
import { InventoryTable } from "@/components/inventory/InventoryTable";
import { useInventoryWatch } from "@/hooks/useInventoryWatch";
import {
  filtersFromSearch,
  filtersToQuery,
  inventoryQueryKey,
  searchFromFilters,
  useInventoryStore,
  type InventoryFilters,
} from "@/store/inventory";
import { useSelectionStore } from "@/store/selection";

export function InventoryPage() {
  const navigate = useNavigate({ from: "/inventory" });
  const rawSearch = useSearch({ from: "/inventory" });
  const filters = filtersFromSearch(rawSearch as Record<string, unknown>);

  const columnVisibility = useInventoryStore((state) => state.columnVisibility);
  const hydrateColumnVisibility = useInventoryStore((state) => state.hydrateColumnVisibility);
  const selectedRowIds = useSelectionStore((state) => state.selectedRowIds);
  const openDrawerId = useSelectionStore((state) => state.openDrawerId);
  const setOpenDrawerId = useSelectionStore((state) => state.setOpenDrawerId);
  const toggleRowSelection = useSelectionStore((state) => state.toggleRowSelection);

  useEffect(() => {
    hydrateColumnVisibility();
  }, [hydrateColumnVisibility]);

  useInventoryWatch();

  const { data, isLoading, isError, error } = useQuery({
    queryKey: inventoryQueryKey(filters),
    queryFn: () => fetchInventorySummary(filtersToQuery(filters)),
  });

  const selectedItem = useMemo(
    () => data?.items.find((item) => item.uid === openDrawerId) ?? null,
    [data?.items, openDrawerId],
  );

  const yamlSnippet = selectedItem ? buildInventoryItemYamlSnippet(selectedItem) : "";

  function updateFilters(patch: Partial<InventoryFilters>) {
    const next = { ...filters, ...patch, offset: 0 };
    void navigate({
      search: searchFromFilters(next),
      replace: true,
    });
  }

  function handleRowClick(item: Item) {
    toggleRowSelection(item.uid);
    setOpenDrawerId(item.uid);
  }

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-2xl font-semibold text-kollect-navy">Inventory</h1>
        <p className="text-sm text-slate-600">
          Collected resource rows from the Read API. Filters sync to the URL.
        </p>
      </header>

      <InventoryFilterBar filters={filters} onChange={updateFilters} />

      {isLoading && <p className="text-sm text-slate-500">Loading inventory…</p>}
      {isError && (
        <p
          className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800"
          role="alert"
        >
          {(error as Error).message}
        </p>
      )}

      {data && (
        <>
          <ExportStatusBar statuses={data.exportStatus} />
          {data.items.length === 0 ? (
            <p className="rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600">
              No inventory rows match the current filters.
            </p>
          ) : (
            <InventoryTable
              items={data.items}
              columnVisibility={columnVisibility}
              selectedRowIds={selectedRowIds}
              onRowClick={handleRowClick}
            />
          )}
          <p className="text-xs text-slate-500">
            schemaVersion {data.schemaVersion} · {data.itemCount} row(s)
            {data.pagination?.hasMore ? " · more rows available" : ""}
          </p>
        </>
      )}

      <DetailDrawer
        open={selectedItem !== null}
        onOpenChange={(open) => {
          if (!open) {
            setOpenDrawerId(null);
          }
        }}
        title={
          selectedItem
            ? `${selectedItem.kind}/${selectedItem.name}`
            : "Inventory item"
        }
        subtitle={
          selectedItem
            ? `${selectedItem.namespace} · collected by ${selectedItem.targetNamespace}/${selectedItem.targetName}`
            : undefined
        }
        yamlSnippet={yamlSnippet}
      />
    </div>
  );
}
