import { beforeEach, describe, expect, it } from "vitest";
import { useSelectionStore } from "./selection";

describe("useSelectionStore", () => {
  beforeEach(() => {
    useSelectionStore.getState().reset();
  });

  it("toggles row selection membership", () => {
    useSelectionStore.getState().toggleRowSelection("uid-1");
    expect(useSelectionStore.getState().selectedRowIds).toEqual(["uid-1"]);

    useSelectionStore.getState().toggleRowSelection("uid-1");
    expect(useSelectionStore.getState().selectedRowIds).toEqual([]);
  });

  it("replaces selected row ids", () => {
    useSelectionStore.getState().setSelectedRowIds(["a", "b"]);
    expect(useSelectionStore.getState().selectedRowIds).toEqual(["a", "b"]);
  });

  it("tracks open drawer id and clears selection", () => {
    useSelectionStore.getState().setOpenDrawerId("uid-9");
    useSelectionStore.getState().setSelectedRowIds(["uid-9"]);
    useSelectionStore.getState().clearSelection();
    expect(useSelectionStore.getState().openDrawerId).toBeNull();
    expect(useSelectionStore.getState().selectedRowIds).toEqual([]);
  });

  it("reset restores initial state", () => {
    useSelectionStore.getState().setOpenDrawerId("x");
    useSelectionStore.getState().setSelectedRowIds(["x"]);
    useSelectionStore.getState().reset();
    expect(useSelectionStore.getState().openDrawerId).toBeNull();
    expect(useSelectionStore.getState().selectedRowIds).toEqual([]);
  });
});
