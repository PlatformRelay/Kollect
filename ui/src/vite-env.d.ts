/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_READ_API_URL?: string;
  readonly VITE_K8S_API_URL?: string;
  readonly VITE_ENABLE_MSW?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
