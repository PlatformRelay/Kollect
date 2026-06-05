import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from "@tanstack/react-router";
import { AppShell } from "./components/AppShell";
import { OverviewPage } from "./routes/overview";
import { InventoryPage } from "./routes/inventory";
import { TargetsPage } from "./routes/targets";
import { SinksPage } from "./routes/sinks";

const rootRoute = createRootRoute({
  component: () => (
    <AppShell>
      <Outlet />
    </AppShell>
  ),
});

const overviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: OverviewPage,
});

const inventoryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/inventory",
  component: InventoryPage,
});

const targetsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/targets",
  component: TargetsPage,
});

const sinksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/sinks",
  component: SinksPage,
});

const routeTree = rootRoute.addChildren([
  overviewRoute,
  inventoryRoute,
  targetsRoute,
  sinksRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
