import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { OverviewPage } from "./overview";

function renderOverview() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <OverviewPage />
    </QueryClientProvider>,
  );
}

describe("OverviewPage", () => {
  it("shows degraded strip and export-enriched stat cards from MSW fixtures", async () => {
    renderOverview();

    expect(await screen.findByRole("region", { name: /degraded resources/i })).toBeInTheDocument();
    expect(screen.getByText(/legacy-batch/i)).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("120")).toBeInTheDocument();
    });

    expect(screen.getByText(/last export/i)).toBeInTheDocument();
    expect(screen.getByText(/1 degraded sink/i)).toBeInTheDocument();
  });
});
