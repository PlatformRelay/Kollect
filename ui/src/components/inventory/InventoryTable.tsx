import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useMemo, useRef } from "react";
import type { Item } from "@/api/inventory";
import type { ColumnVisibility } from "@/store/inventory";

const columnHelper = createColumnHelper<Item>();
const ROW_HEIGHT = 40;

type InventoryTableProps = {
  items: Item[];
  columnVisibility: ColumnVisibility;
  selectedRowIds: string[];
  onRowClick?: (item: Item) => void;
};

function buildColumns(visibility: ColumnVisibility): ColumnDef<Item, string>[] {
  const columns: ColumnDef<Item, string>[] = [];

  if (visibility.namespace) {
    columns.push(columnHelper.accessor("namespace", { header: "Namespace" }));
  }
  if (visibility.kind) {
    columns.push(
      columnHelper.accessor("kind", {
        header: "Kind",
        cell: (info) => <span className="font-mono text-xs">{info.getValue()}</span>,
      }),
    );
  }
  if (visibility.name) {
    columns.push(columnHelper.accessor("name", { header: "Name" }));
  }
  if (visibility.target) {
    columns.push(
      columnHelper.display({
        id: "target",
        header: "Target",
        cell: ({ row }) => (
          <span className="text-slate-600">
            {row.original.targetNamespace}/{row.original.targetName}
          </span>
        ),
      }),
    );
  }
  if (visibility.group) {
    columns.push(
      columnHelper.accessor((row) => row.group ?? "—", {
        id: "group",
        header: "Group",
      }),
    );
  }
  if (visibility.version) {
    columns.push(columnHelper.accessor("version", { header: "Version" }));
  }

  return columns;
}

export function InventoryTable({
  items,
  columnVisibility,
  selectedRowIds,
  onRowClick,
}: InventoryTableProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const columns = useMemo(() => buildColumns(columnVisibility), [columnVisibility]);

  const table = useReactTable({
    data: items,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  const rowVirtualizer = useVirtualizer({
    count: table.getRowModel().rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8,
  });

  const virtualRows = rowVirtualizer.getVirtualItems();
  const rows = table.getRowModel().rows;
  const useFallback = virtualRows.length === 0 && rows.length > 0;
  const renderedRows = useFallback
    ? rows.map((row, index) => ({ row, virtualRow: { index, start: index * ROW_HEIGHT } }))
    : virtualRows.map((virtualRow) => ({
        row: rows[virtualRow.index]!,
        virtualRow,
      }));
  const totalSize = useFallback ? rows.length * ROW_HEIGHT : rowVirtualizer.getTotalSize();
  const paddingTop = !useFallback && virtualRows.length > 0 ? virtualRows[0].start : 0;
  const paddingBottom =
    !useFallback && virtualRows.length > 0
      ? totalSize - virtualRows[virtualRows.length - 1].end
      : 0;

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
      <div
        ref={parentRef}
        className="max-h-[28rem] overflow-auto"
        role="grid"
        aria-rowcount={items.length}
        aria-label="Inventory rows"
      >
        <table className="min-w-full text-left text-sm">
          <thead className="sticky top-0 z-10 border-b border-slate-200 bg-slate-50 text-xs uppercase text-slate-500">
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th key={header.id} scope="col" className="px-4 py-2 font-medium">
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {paddingTop > 0 ? (
              <tr aria-hidden="true">
                <td colSpan={columns.length} style={{ height: paddingTop }} />
              </tr>
            ) : null}
            {renderedRows.map(({ row, virtualRow }) => {
              const item = row.original;
              const selected = selectedRowIds.includes(item.uid);

              return (
                <tr
                  key={row.id}
                  role="row"
                  aria-rowindex={virtualRow.index + 1}
                  aria-selected={selected}
                  className={`border-b border-slate-100 last:border-0 ${
                    selected ? "bg-sky-50" : "hover:bg-slate-50"
                  } ${onRowClick ? "cursor-pointer" : ""}`}
                  onClick={onRowClick ? () => onRowClick(item) : undefined}
                  onKeyDown={
                    onRowClick
                      ? (event) => {
                          if (event.key === "Enter" || event.key === " ") {
                            event.preventDefault();
                            onRowClick(item);
                          }
                        }
                      : undefined
                  }
                  tabIndex={onRowClick ? 0 : undefined}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-4 py-2">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              );
            })}
            {paddingBottom > 0 ? (
              <tr aria-hidden="true">
                <td colSpan={columns.length} style={{ height: paddingBottom }} />
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}
