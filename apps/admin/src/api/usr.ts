import { API_BASE_URL } from './config'
import { ApiError, apiFetch } from './http'
import { clearSession, setCredentials, setToken } from './auth-session'
import type {
  ErrorRep,
  UsrBootstrapStatusRep,
  UsrCreateRep,
  UsrCreateReq,
  UsrLoginRep,
  UsrMain,
  UsrUpdateProfileReq,
} from './types'

/**
 * Authenticate and persist the session.
 * Stores both the JWT and the credentials (for silent renewal).
 */
export async function login(username: string, password: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/usr/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })

  const payload = (await response.json().catch(() => ({}))) as Partial<
    UsrLoginRep & ErrorRep
  >
  if (!response.ok || !payload.jwt) {
    throw new ApiError(
      payload.message || 'Login failed',
      payload.code || 'not_authorized',
      response.status,
    )
  }

  setToken(payload.jwt)
  setCredentials(username, password)
}

export function logout(): void {
  clearSession()
}

export function getProfile(): Promise<UsrMain> {
  return apiFetch<UsrMain>('/usr/profile')
}

export function getBootstrapStatus(): Promise<UsrBootstrapStatusRep> {
  return apiFetch<UsrBootstrapStatusRep>('/usr/bootstrap/status')
}

export function createUser(req: UsrCreateReq): Promise<UsrCreateRep> {
  return apiFetch<UsrCreateRep>('/usr', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateProfile(req: UsrUpdateProfileReq): Promise<void> {
  return apiFetch<void>('/usr/profile', {
    method: 'PATCH',
    body: JSON.stringify(req),
  })
}
