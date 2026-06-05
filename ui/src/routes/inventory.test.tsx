import { describe, expect, it, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "@/test/setup";
import { renderInventoryPage } from "@/test/inventory-utils";
import { useSelectionStore } from "@/store/selection";

describe("InventoryPage", () => {
  beforeEach(() => {
    useSelectionStore.getState().reset();
  });

  it("shows inventory rows from the Read API", async () => {
    await renderInventoryPage();

    expect(await screen.findByRole("grid", { name: "Inventory rows" })).toBeInTheDocument();
    expect(screen.getByText("web")).toBeInTheDocument();
  });

  it("renders export status chips", async () => {
    await renderInventoryPage();

    expect(await screen.findByRole("region", { name: "Export status" })).toBeInTheDocument();
    expect(screen.getByText("team-a/git-main")).toBeInTheDocument();
    expect(screen.getByText("team-a/s3-backup")).toBeInTheDocument();
  });

  it("shows empty state when filters match nothing", async () => {
    await renderInventoryPage({ initialEntries: ["/inventory?kind=NoSuchKind"] });

    expect(
      await screen.findByText("No inventory rows match the current filters."),
    ).toBeInTheDocument();
  });

  it("shows error alert on API failure", async () => {
    server.use(
      http.get("/v1alpha1/inventory", () =>
        HttpResponse.json({ message: "boom" }, { status: 500 }),
      ),
    );

    await renderInventoryPage();

    expect(await screen.findByRole("alert")).toHaveTextContent("Read API 500");
  });

  it("updates URL search when filters change", async () => {
    const user = userEvent.setup();
    const { router } = await renderInventoryPage();

    const kindInput = await screen.findByRole("textbox", { name: /kind/i });
    await user.clear(kindInput);
    await user.type(kindInput, "Deployment");

    await waitFor(() => {
      expect(router.state.location.search).toMatchObject({ kind: "Deployment" });
    });
  });

  it("selects a row and opens the detail drawer", async () => {
    const user = userEvent.setup();
    await renderInventoryPage();

    const cell = await screen.findByText("web");
    const row = cell.closest("tr");
    expect(row).not.toBeNull();
    await user.click(row!);

    expect(useSelectionStore.getState().openDrawerId).toBe("uid-web-001");
    expect(useSelectionStore.getState().selectedRowIds).toContain("uid-web-001");
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Deployment/web")).toBeInTheDocument();
  });
});
