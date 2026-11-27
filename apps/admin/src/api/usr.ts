import { API_BASE_URL } from './config'
import { ApiError, apiFetch } from './http'
import { buildListQuery } from './query'
import { setSession, timeoutSignal } from './auth-session'
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

/** Plain unauthenticated POST that surfaces gateway errors as `ApiError`. */
async function postPublic<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const payload = (await response.json().catch(() => ({}))) as Partial<T> &
    Partial<ErrorRep>
  if (!response.ok) {
    throw new ApiError(
      payload.message || 'Request failed',
      payload.code || 'service_not_available',
      response.status,
      payload.fields,
    )
  }
  return payload as T
}

/** Authenticate and persist the issued token pair. */
export async function login(username: string, password: string): Promise<UsrLoginRep> {
  const rep = await postPublic<UsrLoginRep>('/usr/login', { username, password })
  if (rep.jwt) {
    setSession({ jwt: rep.jwt, refresh_token: rep.refresh_token ?? '' })
  }
  return rep
}

/** How long sign-out waits for the server before giving up. */
const LOGOUT_TIMEOUT_MS = 5000

/**
 * Sign out on the server: revokes every token of the current user, so the
 * session ends on all devices. Only the server side — dropping the local
 * session is the caller's job (see `useAuthStore().logout`), and it must
 * happen whether or not this call succeeds.
 *
 * An expired access token is renewed once so the revocation still reaches the
 * server; a dead session is left to the caller instead of raising
 * `auth:required`.
 */
export function logout(): Promise<void> {
  return apiFetch<void>(
    '/usr/logout',
    {
      method: 'POST',
      body: JSON.stringify({}),
      signal: timeoutSignal(LOGOUT_TIMEOUT_MS),
    },
    { keepSessionOnAuthFailure: true },
  )
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

/**
 * Update the own profile. `current_password` is required when `password` or
 * `username` changes (`current_password_invalid` otherwise). A password change
 * revokes every token of the user — the current session ends with it.
 */
export function updateProfile(req: UsrUpdateProfileReq): Promise<void> {
  return apiFetch<void>('/usr/profile', {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

// ── Admin user management ──────────────────────────────────
// `GET /usr` and `GET /usr/{id}` are admin-only (`no_permission` otherwise):
// call them from admin screens only; own data comes from `getProfile`.

export function listUsers(req: UsrListReq = {}): Promise<UsrListRep> {
  const query = buildListQuery(req.list_params, {
    active: req.active,
    is_admin: req.is_admin,
    search: req.search,
    app_id: req.app_id,
  })
  return apiFetch<UsrListRep>(`/usr${query}`)
}

export function getUser(id: number | string): Promise<UsrMain> {
  return apiFetch<UsrMain>(`/usr/${encodeURIComponent(String(id))}`)
}

/**
 * Update a user. An admin cannot change their **own** `username` / `password`
 * here (`no_permission`) — leave both out when `id` is the current user; they
 * are changed in the profile, where the current password is asked.
 */
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
