import { create } from "zustand";

type ConnectionState = {
  readApiBaseUrl: string;
  mockApiEnabled: boolean;
  showDebugBanner: boolean;
  hydrate: () => void;
  setShowDebugBanner: (show: boolean) => void;
};

function resolveMockEnabled(): boolean {
  if (import.meta.env.VITE_MOCK_API === "false") {
    return false;
  }
  if (import.meta.env.VITE_MOCK_API === "true") {
    return true;
  }
  // Backward compat alias (Phase 1)
  return import.meta.env.VITE_ENABLE_MSW === "true";
}

function resolveReadApiBaseUrl(): string {
  return import.meta.env.VITE_READ_API_URL ?? "http://127.0.0.1:8082";
}

function resolveDebugBanner(): boolean {
  if (!import.meta.env.DEV) {
    return false;
  }
  return new URLSearchParams(window.location.search).get("debug") === "true";
}

export const useConnectionStore = create<ConnectionState>((set) => ({
  readApiBaseUrl: resolveReadApiBaseUrl(),
  mockApiEnabled: resolveMockEnabled(),
  showDebugBanner: typeof window !== "undefined" ? resolveDebugBanner() : false,
  hydrate: () =>
    set({
      readApiBaseUrl: resolveReadApiBaseUrl(),
      mockApiEnabled: resolveMockEnabled(),
      showDebugBanner: resolveDebugBanner(),
    }),
  setShowDebugBanner: (show) => set({ showDebugBanner: show }),
}));

export function isMockApiEnabled(): boolean {
  return resolveMockEnabled();
}
