/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_READ_API_URL?: string;
  readonly VITE_K8S_API_URL?: string;
  /** Enable MSW mock Read API (default in task ui-dev). */
  readonly VITE_MOCK_API?: string;
  /** @deprecated Use VITE_MOCK_API — kept for backward compatibility. */
  readonly VITE_ENABLE_MSW?: string;
  /** SSE mock event interval in milliseconds (default 5000). */
  readonly VITE_MOCK_WATCH_INTERVAL_MS?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
