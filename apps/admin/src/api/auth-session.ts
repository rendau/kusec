import { API_BASE_URL } from './config'
import type { UsrLoginRep } from './types'

/**
 * Token & credential persistence for the admin session.
 *
 * The JWT is kept in localStorage so reloads stay authenticated. The login
 * credentials are also stored so a single silent re-login can be attempted
 * when the token expires (see `renewTokenOnce`), mirroring the kusec backend
 * which has no refresh-token endpoint.
 */

const storageKeys = {
  token: 'kusec_admin_token',
  credentials: 'kusec_admin_credentials',
} as const

interface Credentials {
  username: string
  password: string
}

let token = localStorage.getItem(storageKeys.token) ?? ''
let renewInFlight: Promise<boolean> | null = null

function readCredentials(): Credentials | null {
  const raw = localStorage.getItem(storageKeys.credentials)
  if (!raw) {
    return null
  }
  try {
    const parsed = JSON.parse(raw) as Credentials
    if (!parsed.username || !parsed.password) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

function writeCredentials(creds: Credentials | null): void {
  if (!creds) {
    localStorage.removeItem(storageKeys.credentials)
    return
  }
  localStorage.setItem(storageKeys.credentials, JSON.stringify(creds))
}

export function getToken(): string {
  return token
}

export function setToken(value: string): void {
  token = value.trim()
  if (token) {
    localStorage.setItem(storageKeys.token, token)
  } else {
    localStorage.removeItem(storageKeys.token)
  }
}

export function setCredentials(username: string, password: string): void {
  writeCredentials({ username: username.trim(), password })
}

export function clearSession(): void {
  setToken('')
  writeCredentials(null)
}

async function requestLogin(
  username: string,
  password: string,
): Promise<string> {
  const response = await fetch(`${API_BASE_URL}/usr/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })

  const data = (await response.json().catch(() => ({}))) as Partial<UsrLoginRep>
  if (!response.ok || typeof data.jwt !== 'string' || data.jwt.trim() === '') {
    return ''
  }
  return data.jwt
}

/**
 * Attempt a single silent re-login using the stored credentials.
 * Concurrent callers share the same in-flight request.
 */
export async function renewTokenOnce(): Promise<boolean> {
  if (renewInFlight) {
    return renewInFlight
  }

  const credentials = readCredentials()
  if (!credentials) {
    return false
  }

  renewInFlight = requestLogin(credentials.username, credentials.password)
    .then((jwt) => {
      if (!jwt) {
        return false
      }
      setToken(jwt)
      return true
    })
    .finally(() => {
      renewInFlight = null
    })

  return renewInFlight
}
