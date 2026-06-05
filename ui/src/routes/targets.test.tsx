import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TargetsPage } from "./targets";
import { SinksPage } from "./sinks";

function renderWithQuery(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

describe("TargetsPage", () => {
  it("opens detail drawer when a target row is clicked", async () => {
    const user = userEvent.setup();
    renderWithQuery(<TargetsPage />);

    const row = await screen.findByRole("button", { name: "team-a/legacy-batch" });
    await user.click(row);

    expect(await screen.findByRole("dialog", { name: /legacy-batch/i })).toBeInTheDocument();
    expect(screen.getByText("ReconcileError")).toBeInTheDocument();
  });

  it("shows health badges in the targets table", async () => {
    renderWithQuery(<TargetsPage />);

    await screen.findByRole("table", { name: "Targets" });
    expect(screen.getAllByRole("status").length).toBeGreaterThan(0);
  });
});

describe("SinksPage", () => {
  it("opens detail drawer when a sink row is clicked", async () => {
    const user = userEvent.setup();
    renderWithQuery(<SinksPage />);

    const row = await screen.findByRole("button", { name: "team-a/s3-backup" });
    await user.click(row);

    expect(await screen.findByRole("dialog", { name: /s3-backup/i })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("Resource YAML snippet")).toHaveTextContent("s3-backup");
    });
    expect(screen.getByLabelText("Resource YAML snippet")).toHaveTextContent("KollectSink");
  });
});
