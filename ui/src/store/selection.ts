import { create } from "zustand";

type SelectionState = {
  openDrawerId: string | null;
  selectedRowIds: string[];
  setOpenDrawerId: (id: string | null) => void;
  toggleRowSelection: (id: string) => void;
  setSelectedRowIds: (ids: string[]) => void;
  clearSelection: () => void;
  reset: () => void;
};

const initialState = {
  openDrawerId: null as string | null,
  selectedRowIds: [] as string[],
};

export const useSelectionStore = create<SelectionState>((set, get) => ({
  ...initialState,
  setOpenDrawerId: (id) => set({ openDrawerId: id }),
  toggleRowSelection: (id) => {
    const current = get().selectedRowIds;
    if (current.includes(id)) {
      set({ selectedRowIds: current.filter((rowId) => rowId !== id) });
      return;
    }
    set({ selectedRowIds: [...current, id] });
  },
  setSelectedRowIds: (ids) => set({ selectedRowIds: ids }),
  clearSelection: () => set({ openDrawerId: null, selectedRowIds: [] }),
  reset: () => set(initialState),
}));
