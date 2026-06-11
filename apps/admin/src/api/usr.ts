import { API_BASE_URL } from './config'
import { ApiError, apiFetch } from './http'
import { buildListQuery } from './query'
import { clearSession, setSession } from './auth-session'
import type {
  ErrorRep,
  UsrBootstrapStatusRep,
  UsrCreateRep,
  UsrCreateReq,
  UsrListRep,
  UsrListReq,
  UsrLoginRep,
  UsrMain,
  UsrUpdateProfileReq,
  UsrUpdateReq,
} from './types'

/** Authenticate and persist the session (access + refresh token pair). */
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

  setSession({ jwt: payload.jwt, refresh_token: payload.refresh_token ?? '' })
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
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

// ── Admin user management ──────────────────────────────────

export function listUsers(req: UsrListReq = {}): Promise<UsrListRep> {
  const query = buildListQuery(req.list_params, {
    active: req.active,
    is_admin: req.is_admin,
    search: req.search,
  })
  return apiFetch<UsrListRep>(`/usr${query}`)
}

export function getUser(id: number | string): Promise<UsrMain> {
  return apiFetch<UsrMain>(`/usr/${encodeURIComponent(String(id))}`)
}

export function updateUser(id: number | string, req: UsrUpdateReq): Promise<void> {
  return apiFetch<void>(`/usr/${encodeURIComponent(String(id))}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteUser(id: number | string): Promise<void> {
  return apiFetch<void>(`/usr/${encodeURIComponent(String(id))}`, {
    method: 'DELETE',
  })
}
