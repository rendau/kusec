/**
 * Base URL of the kusec gRPC-gateway HTTP API.
 * Empty string → same-origin (paths like `/usr/login`).
 * Set `VITE_API_BASE_URL=http://localhost:9090` for local dev.
 */
export const API_BASE_URL = (
  import.meta.env.VITE_API_BASE_URL ?? ''
).trim()
