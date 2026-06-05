import type { ReactNode } from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import { ConnectionBanner } from "./ConnectionBanner";
import {
  LayoutDashboard,
  Package,
  Target,
  Database,
} from "lucide-react";

const navItems = [
  { to: "/", label: "Overview", icon: LayoutDashboard },
  { to: "/inventory", label: "Inventory", icon: Package },
  { to: "/targets", label: "Targets", icon: Target },
  { to: "/sinks", label: "Sinks", icon: Database },
] as const;

type AppShellProps = {
  children: ReactNode;
};

export function AppShell({ children }: AppShellProps) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  return (
    <div className="min-h-screen flex flex-col">
      <ConnectionBanner />
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center gap-6 px-4 py-3">
          <Link to="/" className="flex items-center gap-3 no-underline">
            <img src="/logo.svg" alt="Kollect" className="h-8 w-8" />
            <span className="text-lg font-semibold text-kollect-navy">Kollect</span>
          </Link>
          <nav className="flex flex-1 gap-1" aria-label="Primary">
            {navItems.map(({ to, label, icon: Icon }) => {
              const active = pathname === to || (to !== "/" && pathname.startsWith(to));
              return (
                <Link
                  key={to}
                  to={to}
                  className={`inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium no-underline ${
                    active
                      ? "bg-kollect-blue/10 text-kollect-blue"
                      : "text-slate-600 hover:bg-slate-100"
                  }`}
                >
                  <Icon className="h-4 w-4" aria-hidden />
                  {label}
                </Link>
              );
            })}
          </nav>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6">{children}</main>
    </div>
  );
}
