import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DegradedResourcesStrip } from "./DegradedResourcesStrip";

describe("DegradedResourcesStrip", () => {
  it("renders nothing when there are no degraded resources", () => {
    const { container } = render(
      <DegradedResourcesStrip
        targets={[
          {
            name: "deploys",
            namespace: "team-a",
            generation: 1,
            observedGeneration: 1,
            conditions: [{ type: "Synced", status: "True" }],
          },
        ]}
        inventories={[]}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("lists degraded resources sorted by severity with accessible region", () => {
    render(
      <DegradedResourcesStrip
        targets={[
          {
            name: "legacy-batch",
            namespace: "team-a",
            generation: 5,
            observedGeneration: 4,
            conditions: [
              {
                type: "Degraded",
                status: "True",
                reason: "PartialSync",
                message: "2 of 5 GVKs failing",
                lastTransitionTime: "2026-06-05T09:15:00Z",
              },
            ],
          },
          {
            name: "denied",
            namespace: "team-b",
            generation: 2,
            observedGeneration: 2,
            conditions: [
              {
                type: "Synced",
                status: "False",
                reason: "ScopeGVKDenied",
                message: "forbidden GVK",
                lastTransitionTime: "2026-06-05T08:00:00Z",
              },
            ],
          },
        ]}
        inventories={[]}
      />,
    );

    const region = screen.getByRole("region", { name: /degraded resources/i });
    expect(region).toBeInTheDocument();
    expect(screen.getByText(/2 degraded/i)).toBeInTheDocument();

    const rows = screen.getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("KollectTarget/legacy-batch");
    expect(rows[0]).toHaveTextContent("PartialSync");
    expect(rows[1]).toHaveTextContent("KollectTarget/denied");
    expect(rows[1]).toHaveTextContent("ScopeGVKDenied");
  });

  it("shows loading skeleton while status is loading", () => {
    render(<DegradedResourcesStrip loading />);
    expect(screen.getByRole("status", { name: /loading degraded resources/i })).toBeInTheDocument();
  });
});
