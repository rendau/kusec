import { API_BASE_URL } from './config'
import { ApiError, UNKNOWN_ERROR_CODE } from './errors'
import type { ErrorRep, UsrLoginRep } from './types'

/**
 * Token persistence and silent renewal for the admin session.
 *
 * The backend issues a pair: a short-lived access JWT and a long-lived
 * refresh token. Both are kept in localStorage so reloads stay
 * authenticated. When a request fails auth (the access token expired),
 * `apiFetch` calls `renewTokenOnce` — a single exchange of the refresh
 * token for a fresh pair — and retries the original request.
 *
 * The session is given up only when the backend refuses the refresh token on
 * the merits (`not_authorized`). A refresh that merely could not be done —
 * the server or its database is down (`service_not_available`), no network, a
 * timeout — says nothing about the session, so the tokens stay and the user
 * can simply try again.
 */

type TokenPair = Pick<UsrLoginRep, 'jwt' | 'refresh_token'>

/**
 * How a silent renewal ended:
 *   - `renewed`     — a fresh pair is stored, repeat the request;
 *   - `rejected`    — the backend refused the refresh token (`not_authorized`)
 *                     or there is none to exchange: the session is over;
 *   - `unavailable` — it could not be found out (`error` says why): the
 *                     session must be left alone.
 */
export type RenewResult =
  | { status: 'renewed' }
  | { status: 'rejected' }
  | { status: 'unavailable'; error: unknown }

const storageKeys = {
  token: 'kusec_admin_token',
  refreshToken: 'kusec_admin_refresh_token',
} as const

/** Every request waits for the renewal, so it must not hang on a dead server. */
const REFRESH_TIMEOUT_MS = 15_000

// Older versions kept plaintext login credentials for silent re-login.
// Purge them: the refresh-token flow made that (unsafe) mechanism obsolete.
localStorage.removeItem('kusec_admin_credentials')

let token = localStorage.getItem(storageKeys.token) ?? ''
let refreshToken = localStorage.getItem(storageKeys.refreshToken) ?? ''
let renewInFlight: Promise<RenewResult> | null = null
let sessionReplacement: Promise<void> | null = null

function persist(key: string, value: string): void {
  if (value) {
    localStorage.setItem(key, value)
  } else {
    localStorage.removeItem(key)
  }
}

export function getToken(): string {
  return token
}

export function setToken(value: string): void {
  token = value.trim()
  persist(storageKeys.token, token)
}

export function setRefreshToken(value: string): void {
  refreshToken = value.trim()
  persist(storageKeys.refreshToken, refreshToken)
}

/** Store a freshly issued access + refresh pair. */
export function setSession(rep: TokenPair): void {
  setToken(rep.jwt)
  setRefreshToken(rep.refresh_token ?? '')
}

export function clearSession(): void {
  setToken('')
  setRefreshToken('')
}

/**
 * `AbortSignal` that fires after `ms`, or `null` where the browser has no
 * `AbortSignal.timeout` (the request then waits for the network stack).
 */
export function timeoutSignal(ms: number): AbortSignal | null {
  return typeof AbortSignal.timeout === 'function' ? AbortSignal.timeout(ms) : null
}

/**
 * Run a call whose answer replaces the session: the backend issues a pair of
 * a new session version and the tokens held so far die the moment it commits
 * (`POST /usr/totp/confirm`).
 *
 * The new pair is stored in the same tick the answer is handed over, and
 * while the call is in flight `apiFetch` holds every other request back
 * (`sessionReplaced`). So nothing goes out with the old pair between the
 * answer and the save, and a request that was already on its way and came
 * back refused is repeated with the new pair instead of spending the dead
 * refresh token — which would end the session.
 *
 * The replacing call itself must run with `replacesSession` so it does not
 * wait for the replacement it is part of.
 */
export async function replaceSession<T extends TokenPair>(
  call: () => Promise<T>,
): Promise<T> {
  let release = (): void => {}
  const pending = new Promise<void>((resolve) => {
    release = resolve
  })
  sessionReplacement = pending
  try {
    const rep = await call()
    if (rep.jwt) setSession(rep)
    return rep
  } finally {
    if (sessionReplacement === pending) sessionReplacement = null
    release()
  }
}

/** Settles once no session replacement is in flight (`null` — none is). */
export function sessionReplaced(): Promise<void> | null {
  return sessionReplacement
}

/**
 * Exchange `currentRefreshToken` for a new pair. Resolves to the pair, or to
 * `'rejected'` when the backend refuses the token on the merits. Everything
 * else is thrown: an error answer with another code, a non-JSON answer of a
 * proxy, a network failure, a timeout.
 */
async function requestRefresh(
  currentRefreshToken: string,
): Promise<TokenPair | 'rejected'> {
  const response = await fetch(`${API_BASE_URL}/usr/token/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: currentRefreshToken }),
    signal: timeoutSignal(REFRESH_TIMEOUT_MS),
  })

  const data = (await response.json().catch(() => ({}))) as Partial<UsrLoginRep> &
    Partial<ErrorRep>

  if (response.ok) {
    if (typeof data.jwt === 'string' && data.jwt.trim() !== '') {
      return { jwt: data.jwt, refresh_token: data.refresh_token ?? '' }
    }
    throw new ApiError(
      'token refresh returned no token',
      UNKNOWN_ERROR_CODE,
      response.status,
    )
  }

  if (data.code === 'not_authorized') {
    return 'rejected'
  }
  throw new ApiError(
    data.message || `Request failed with status ${response.status}`,
    data.code || UNKNOWN_ERROR_CODE,
    response.status,
    data.fields,
  )
}

/**
 * Exchange the refresh token for a fresh access + refresh pair (rotation).
 * Concurrent callers share the same in-flight request. Never rejects — see
 * `RenewResult` for the three ways it can end.
 */
export function renewTokenOnce(): Promise<RenewResult> {
  if (renewInFlight) {
    return renewInFlight
  }
  if (!refreshToken) {
    // Nothing to exchange: the session cannot be continued.
    return Promise.resolve({ status: 'rejected' })
  }

  const exchanged = refreshToken
  renewInFlight = requestRefresh(exchanged)
    .then((outcome): RenewResult => {
      // The session changed while the exchange was in flight (signed out, or
      // replaced by a newer pair): its answer is stale and must not overwrite
      // what is stored now — least of all bring back a session after sign-out.
      if (refreshToken !== exchanged) {
        return token ? { status: 'renewed' } : { status: 'rejected' }
      }
      if (outcome === 'rejected') {
        return { status: 'rejected' }
      }
      setSession(outcome)
      return { status: 'renewed' }
    })
    .catch((error: unknown): RenewResult => ({ status: 'unavailable', error }))
    .finally(() => {
      renewInFlight = null
    })

  return renewInFlight
}
