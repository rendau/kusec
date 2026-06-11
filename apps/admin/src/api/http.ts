import { API_BASE_URL } from './config'
import type { ErrorRep } from './types'
import { clearSession, getToken, renewTokenOnce } from './auth-session'

/**
 * Authenticated fetch wrapper around the kusec gRPC-gateway HTTP API.
 *
 * The gateway returns HTTP 400 for all business errors with a
 * `{ code, message, fields }` body, so auth failures are detected by the
 * `not_authorized` code rather than the HTTP status. On an auth failure
 * (expired access token) the refresh token is exchanged for a fresh pair
 * once and the original request is retried; if the refresh fails too, the
 * session is cleared and an `auth:required` event is dispatched for
 * App.vue to send the user to the login page.
 */

export class ApiError extends Error {
  readonly code: string
  readonly status: number
  readonly fields: Record<string, string>

  constructor(
    message: string,
    code: string,
    status: number,
    fields?: Record<string, string>,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.fields = fields ?? {}
  }
}

/**
 * Human-readable message for any error thrown by the API layer.
 * Use as the single place deciding between server and fallback text.
 */
export function apiErrorMessage(error: unknown, fallback: string): string {
  return error instanceof ApiError ? error.message : fallback
}

function isAuthError(error: ApiError): boolean {
  return error.code === 'not_authorized' || error.status === 401
}

function notifyAuthRequired(): void {
  window.dispatchEvent(new CustomEvent('auth:required'))
}

async function parseApiError(response: Response): Promise<ApiError> {
  const payload = (await response.json().catch(() => ({}))) as Partial<ErrorRep>
  const code = payload.code || 'service_not_available'
  const message = payload.message || `Request failed with status ${response.status}`
  return new ApiError(message, code, response.status, payload.fields)
}

interface FetchOptions {
  /** Retry once after a silent token renewal on auth failure (default true). */
  retryOnAuth?: boolean
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
  options: FetchOptions = {},
): Promise<T> {
  const headers = new Headers(init?.headers ?? {})
  headers.set('Content-Type', 'application/json')

  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, { ...init, headers })

  if (response.ok) {
    if (response.status === 204) {
      return undefined as T
    }
    const text = await response.text()
    return (text ? JSON.parse(text) : {}) as T
  }

  const parsedError = await parseApiError(response)
  const canRetry = options.retryOnAuth !== false && isAuthError(parsedError)
  if (!canRetry) {
    throw parsedError
  }

  const renewed = await renewTokenOnce()
  if (!renewed) {
    clearSession()
    notifyAuthRequired()
    throw parsedError
  }

  return apiFetch<T>(path, init, { retryOnAuth: false })
}
