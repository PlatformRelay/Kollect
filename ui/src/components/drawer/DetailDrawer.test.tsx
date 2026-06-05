import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import type { Condition } from "@/api/k8s-status";
import { DetailDrawer } from "./DetailDrawer";

const legacyConditions: Condition[] = [
  {
    type: "Synced",
    status: "False",
    reason: "ReconcileError",
    message: "RBAC: cannot list cronjobs",
    lastTransitionTime: "2026-06-05T09:15:00Z",
  },
  {
    type: "Degraded",
    status: "True",
    reason: "PartialSync",
    message: "2 of 5 GVKs failing",
    lastTransitionTime: "2026-06-05T09:15:00Z",
  },
];

const yamlSnippet = [
  "apiVersion: kollect.dev/v1alpha1",
  "kind: KollectTarget",
  "metadata:",
  "  name: legacy-batch",
  "  namespace: team-a",
  "spec: {}",
].join("\n");

function DrawerHarness() {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);

  return (
    <div>
      <button ref={triggerRef} type="button" onClick={() => setOpen(true)}>
        Open legacy-batch
      </button>
      <button type="button">Outside action</button>
      <DetailDrawer
        open={open}
        onOpenChange={setOpen}
        returnFocusRef={triggerRef}
        title="legacy-batch"
        subtitle="team-a"
        conditions={legacyConditions}
        generation={5}
        observedGeneration={4}
        yamlSnippet={yamlSnippet}
      />
    </div>
  );
}

describe("DetailDrawer", () => {
  it("traps keyboard focus inside the panel while open", async () => {
    const user = userEvent.setup();
    render(<DrawerHarness />);

    await user.click(screen.getByRole("button", { name: "Open legacy-batch" }));

    const dialog = await screen.findByRole("dialog", { name: /legacy-batch/i });
    const closeButton = within(dialog).getByRole("button", { name: "Close" });
    const copyButton = within(dialog).getByRole("button", { name: /copy yaml/i });

    closeButton.focus();
    expect(closeButton).toHaveFocus();

    await user.tab();
    expect(copyButton).toHaveFocus();

    await user.tab();
    expect(closeButton).toHaveFocus();
  });

  it("returns focus to the row trigger after Escape closes the drawer", async () => {
    const user = userEvent.setup();
    render(<DrawerHarness />);

    const trigger = screen.getByRole("button", { name: "Open legacy-batch" });
    await user.click(trigger);

    await screen.findByRole("dialog", { name: /legacy-batch/i });
    await user.keyboard("{Escape}");

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    expect(trigger).toHaveFocus();
  });

  it("renders conditions table and read-only YAML snippet", async () => {
    render(<DrawerHarness />);

    fireEvent.click(screen.getByRole("button", { name: "Open legacy-batch" }));

    expect(await screen.findByRole("table", { name: "Conditions" })).toBeInTheDocument();
    expect(screen.getByText("ReconcileError")).toBeInTheDocument();
    expect(screen.getByLabelText("Resource YAML snippet")).toHaveTextContent("KollectTarget");
    expect(screen.getByLabelText("Resource YAML snippet")).toHaveTextContent("legacy-batch");
    expect(screen.queryByRole("button", { name: /apply/i })).not.toBeInTheDocument();
  });
});
