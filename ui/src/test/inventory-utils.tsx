import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from "@tanstack/react-router";
import { render, type RenderOptions } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { AppShell } from "@/components/AppShell";
import { InventoryPage } from "@/routes/inventory";
import { filtersFromSearch } from "@/store/inventory";

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

const rootRoute = createRootRoute({
  component: () => (
    <AppShell>
      <Outlet />
    </AppShell>
  ),
});

const inventoryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/inventory",
  validateSearch: (search) => filtersFromSearch(search),
  component: InventoryPage,
});

const routeTree = rootRoute.addChildren([inventoryRoute]);

type WrapperProps = {
  children: ReactNode;
};

export async function renderInventoryPage(
  options?: Omit<RenderOptions, "wrapper"> & { initialEntries?: string[] },
) {
  const queryClient = createTestQueryClient();
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({
      initialEntries: options?.initialEntries ?? ["/inventory"],
    }),
    context: { queryClient },
  });

  function Wrapper() {
    return (
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );
  }

  await router.load();

  return {
    ...render(<Wrapper />, { ...options }),
    router,
    queryClient,
  };
}

export function renderWithQuery(ui: ReactElement, options?: Omit<RenderOptions, "wrapper">) {
  const queryClient = createTestQueryClient();

  function Wrapper({ children }: WrapperProps) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  }

  return render(ui, { wrapper: Wrapper, ...options });
}
