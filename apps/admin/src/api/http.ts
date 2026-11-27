import { API_BASE_URL } from './config'
import { ApiError, UNKNOWN_ERROR_CODE } from './errors'
import type { ErrorRep } from './types'
import { clearSession, getToken, renewTokenOnce } from './auth-session'

export { ApiError, apiErrorMessage } from './errors'

/**
 * Authenticated fetch wrapper around the kusec gRPC-gateway HTTP API.
 *
 * The gateway returns HTTP 400 for all business errors with a
 * `{ code, message, fields }` body, so auth failures are detected by the
 * `not_authorized` code rather than the HTTP status. On an auth failure
 * (expired access token) the refresh token is exchanged for a fresh pair
 * once and the original request is retried.
 *
 * The backend checks the user on every request, so `not_authorized` also
 * means "disabled" or "signed out everywhere" — the refresh is refused then.
 * The session ends in exactly one case: the backend refuses the refresh
 * token on the merits. Then it is cleared and an `auth:required` event is
 * dispatched for App.vue to send the user to the login page.
 *
 * Everything that is not such a refusal leaves the session alone and surfaces
 * as an ordinary request error the user can retry: the request or the refresh
 * failing with `service_not_available` (the backend answers so when its
 * database is down during the session check), a network error, a timeout.
 *
 * There is at most one renewal and one retry per request, and the retry
 * never renews again — so this cannot loop.
 */

function isAuthError(error: ApiError): boolean {
  return error.code === 'not_authorized' || error.status === 401
}

function notifyAuthRequired(): void {
  window.dispatchEvent(new CustomEvent('auth:required'))
}

async function parseApiError(response: Response): Promise<ApiError> {
  const payload = (await response.json().catch(() => ({}))) as Partial<ErrorRep>
  const code = payload.code || UNKNOWN_ERROR_CODE
  const message = payload.message || `Request failed with status ${response.status}`
  return new ApiError(message, code, response.status, payload.fields)
}

interface FetchOptions {
  /** Retry once after a silent token renewal on auth failure (default true). */
  retryOnAuth?: boolean
  /**
   * Leave the session alone even when the refresh token is refused: no
   * clearing, no `auth:required`. For sign-out, where the caller drops the
   * session itself and a dead token is not news.
   */
  keepSessionOnAuthFailure?: boolean
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
  // Not an auth failure, or already the one retry: an ordinary error. A retry
  // refused with a fresh token does not end the session either — only the
  // refresh endpoint decides that, and the next request will ask it.
  if (!isAuthError(parsedError) || options.retryOnAuth === false) {
    throw parsedError
  }

  // The token has changed since this request left (another request renewed
  // it, the user signed in again): repeat with the
  // current one rather than spend a refresh.
  const current = getToken()
  if (current && current !== token) {
    return apiFetch<T>(path, init, { ...options, retryOnAuth: false })
  }

  const renewal = await renewTokenOnce()
  if (renewal.status === 'renewed') {
    return apiFetch<T>(path, init, { ...options, retryOnAuth: false })
  }
  if (renewal.status === 'unavailable') {
    // Whether the session is still valid could not be found out — keep it and
    // report why the renewal failed (that, not "not authorized", is the news).
    throw renewal.error
  }

  // The refresh token was refused on the merits: the session is over. Skipped
  // when the stored token is no longer the one this request used — the
  // session was already cleared by a parallel request, or is a newer one.
  if (!options.keepSessionOnAuthFailure && getToken() === token) {
    clearSession()
    notifyAuthRequired()
  }
  throw parsedError
}
