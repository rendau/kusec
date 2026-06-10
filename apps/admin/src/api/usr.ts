import { API_BASE_URL } from './config'
import { ApiError, apiFetch } from './http'
import { clearSession, setCredentials, setToken } from './auth-session'
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
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

// ── Admin user management ──────────────────────────────────

/** Flatten the (possibly nested) list request into gateway query params. */
function buildUserListQuery(req: UsrListReq): string {
  const params = new URLSearchParams()
  const lp = req.list_params
  if (lp) {
    if (lp.page != null) params.set('list_params.page', String(lp.page))
    if (lp.page_size != null)
      params.set('list_params.page_size', String(lp.page_size))
    if (lp.with_total_count != null)
      params.set('list_params.with_total_count', String(lp.with_total_count))
    if (lp.sort_name) params.set('list_params.sort_name', lp.sort_name)
    for (const s of lp.sort ?? []) params.append('list_params.sort', s)
  }
  if (req.active != null) params.set('active', String(req.active))
  if (req.is_admin != null) params.set('is_admin', String(req.is_admin))
  if (req.search) params.set('search', req.search)

  const query = params.toString()
  return query ? `?${query}` : ''
}

export function listUsers(req: UsrListReq = {}): Promise<UsrListRep> {
  return apiFetch<UsrListRep>(`/usr${buildUserListQuery(req)}`)
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
