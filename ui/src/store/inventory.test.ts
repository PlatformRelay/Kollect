import { beforeEach, describe, expect, it } from "vitest";
import {
  DEFAULT_COLUMN_VISIBILITY,
  filtersFromSearch,
  filtersToQuery,
  inventoryQueryKey,
  searchFromFilters,
  useInventoryStore,
} from "./inventory";

describe("inventory search helpers", () => {
  it("parses filter fields from raw search params", () => {
    expect(
      filtersFromSearch({
        namespace: "team-a",
        kind: "Deployment",
        target: "deploys",
        search: "api",
        limit: "100",
        offset: "10",
        junk: true,
      }),
    ).toEqual({
      namespace: "team-a",
      kind: "Deployment",
      target: "deploys",
      search: "api",
      limit: 100,
      offset: 10,
    });
  });

  it("drops empty values when building router search", () => {
    expect(
      searchFromFilters({
        namespace: "team-a",
        kind: "",
        search: "api",
        limit: 500,
        offset: 0,
      }),
    ).toEqual({
      namespace: "team-a",
      search: "api",
      limit: 500,
      offset: 0,
    });
  });

  it("maps filters to Read API query with default pagination", () => {
    expect(filtersToQuery({ namespace: "team-a", search: "api" })).toEqual({
      namespace: "team-a",
      name: "api",
      limit: 500,
      offset: 0,
    });
  });

  it("builds stable query keys from active filters", () => {
    expect(inventoryQueryKey({ namespace: "team-a" })).toEqual([
      "inventory",
      "list",
      { namespace: "team-a" },
    ]);
  });
});

describe("useInventoryStore column visibility", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useInventoryStore.getState().resetColumnVisibility();
  });

  it("persists column visibility toggles", () => {
    useInventoryStore.getState().setColumnVisibility({ group: true, version: true });
    expect(useInventoryStore.getState().columnVisibility.group).toBe(true);
    expect(useInventoryStore.getState().columnVisibility.version).toBe(true);

    useInventoryStore.getState().hydrateColumnVisibility();
    expect(useInventoryStore.getState().columnVisibility.group).toBe(true);
  });

  it("resets to defaults", () => {
    useInventoryStore.getState().setColumnVisibility({ kind: false });
    useInventoryStore.getState().resetColumnVisibility();
    expect(useInventoryStore.getState().columnVisibility).toEqual(
      DEFAULT_COLUMN_VISIBILITY,
    );
  });
});
