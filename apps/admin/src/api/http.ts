/**
 * Minimal fetch wrapper around the kusec gRPC-gateway HTTP API.
 * Feature modules build their typed clients on top of `request`.
 */
const baseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

export class HttpError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly body?: unknown,
  ) {
    super(message)
    this.name = 'HttpError'
  }
}

export async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(`${baseURL}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init.headers,
    },
  })

  const isJson = response.headers
    .get('content-type')
    ?.includes('application/json')
  const payload = isJson ? await response.json() : await response.text()

  if (!response.ok) {
    const message =
      typeof payload === 'object' && payload !== null && 'message' in payload
        ? String((payload as { message: unknown }).message)
        : response.statusText
    throw new HttpError(response.status, message, payload)
  }

  return payload as T
}
