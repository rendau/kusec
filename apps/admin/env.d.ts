/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the kusec gRPC-gateway HTTP API. */
  readonly VITE_API_BASE_URL: string
  /** Dev/preview server port. */
  readonly VITE_PORT: string
  /** Dev/preview server host. */
  readonly VITE_HOST: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
