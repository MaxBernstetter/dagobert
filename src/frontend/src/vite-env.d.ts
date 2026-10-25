/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_EXPERIMENT_API_URL?: string;
  readonly VITE_DEV_API_PROXY?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
