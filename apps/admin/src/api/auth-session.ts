import { API_BASE_URL } from './config'
import type { UsrLoginRep } from './types'

/**
 * Token persistence and silent renewal for the admin session.
 *
 * The backend issues a pair: a short-lived access JWT and a long-lived
 * refresh token. Both are kept in localStorage so reloads stay
 * authenticated. When a request fails auth (the access token expired),
 * `apiFetch` calls `renewTokenOnce` — a single exchange of the refresh
 * token for a fresh pair — and retries the original request. If the
 * refresh token is rejected too, the session is cleared and the user is
 * sent back to the login page.
 */

const storageKeys = {
  token: 'kusec_admin_token',
  refreshToken: 'kusec_admin_refresh_token',
} as const

// Older versions kept plaintext login credentials for silent re-login.
// Purge them: the refresh-token flow made that (unsafe) mechanism obsolete.
localStorage.removeItem('kusec_admin_credentials')

let token = localStorage.getItem(storageKeys.token) ?? ''
let refreshToken = localStorage.getItem(storageKeys.refreshToken) ?? ''
let renewInFlight: Promise<boolean> | null = null

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
export function setSession(rep: Pick<UsrLoginRep, 'jwt' | 'refresh_token'>): void {
  setToken(rep.jwt)
  setRefreshToken(rep.refresh_token ?? '')
}

export function clearSession(): void {
  setToken('')
  setRefreshToken('')
}

async function requestRefresh(currentRefreshToken: string): Promise<UsrLoginRep | null> {
  const response = await fetch(`${API_BASE_URL}/usr/token/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: currentRefreshToken }),
  })

  const data = (await response.json().catch(() => ({}))) as Partial<UsrLoginRep>
  if (!response.ok || typeof data.jwt !== 'string' || data.jwt.trim() === '') {
    return null
  }
  return { jwt: data.jwt, refresh_token: data.refresh_token ?? '' }
}

/**
 * Exchange the refresh token for a fresh access + refresh pair (rotation).
 * Concurrent callers share the same in-flight request.
 */
export async function renewTokenOnce(): Promise<boolean> {
  if (renewInFlight) {
    return renewInFlight
  }
  if (!refreshToken) {
    return false
  }

  renewInFlight = requestRefresh(refreshToken)
    .then((pair) => {
      if (!pair) {
        return false
      }
      setSession(pair)
      return true
    })
    .catch(() => false)
    .finally(() => {
      renewInFlight = null
    })

  return renewInFlight
}
