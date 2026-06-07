import { expect, test } from "@playwright/test";

test.describe("kollect-ui smoke", () => {
  test("primary navigation renders", async ({ page }) => {
    await page.goto("/");

    await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Inventory" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Targets" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Sinks" })).toBeVisible();
  });

  test("inventory page loads rows under MSW", async ({ page }) => {
    await page.goto("/inventory");

    await expect(page.getByRole("heading", { name: "Inventory" })).toBeVisible();
    await expect(page.getByRole("region", { name: "Export status" })).toBeVisible();
    await expect(page.getByRole("grid", { name: "Inventory rows" })).toBeVisible();
    await expect(
      page.getByRole("cell", { name: "web", exact: true }),
    ).toBeVisible();
  });
});
