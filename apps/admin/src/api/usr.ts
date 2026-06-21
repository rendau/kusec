import { API_BASE_URL } from './config'
import { ApiError, apiFetch } from './http'
import { buildListQuery } from './query'
import { clearSession, setSession } from './auth-session'
import type {
  ErrorRep,
  UsrBootstrapStatusRep,
  UsrCreateRep,
  UsrCreateReq,
  UsrEnrollTotpRep,
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

/**
 * Authenticate. Returns the raw `UsrLoginRep` so the caller can branch on the
 * 2FA flags. The session is persisted here only when a token pair is issued;
 * a 2FA continuation (`totp_required` / `totp_setup_required`) is a non-error
 * 200 response and leaves the session untouched.
 */
export async function login(
  username: string,
  password: string,
  totpCode = '',
): Promise<UsrLoginRep> {
  const rep = await postPublic<UsrLoginRep>('/usr/login', {
    username,
    password,
    totp_code: totpCode,
  })
  if (rep.jwt) {
    setSession({ jwt: rep.jwt, refresh_token: rep.refresh_token ?? '' })
  }
  return rep
}

// ── 2FA (TOTP) ─────────────────────────────────────────────

/**
 * Generate a new TOTP secret + otpauth URL to bind. Pass `setupToken` to enrol
 * right after login (mandatory-admin flow, no session yet); omit it to enrol
 * voluntarily from the profile while authenticated.
 */
export function enrollTotp(setupToken = ''): Promise<UsrEnrollTotpRep> {
  if (setupToken) {
    return postPublic<UsrEnrollTotpRep>('/usr/totp/enroll', { setup_token: setupToken })
  }
  return apiFetch<UsrEnrollTotpRep>('/usr/totp/enroll', {
    method: 'POST',
    body: JSON.stringify({ setup_token: '' }),
  })
}

/**
 * Confirm enrolment with the first code, enabling 2FA. Returns a fresh token
 * pair (persisted), so the user ends up with a full session afterwards.
 */
export async function confirmTotp(code: string, setupToken = ''): Promise<UsrLoginRep> {
  const rep = setupToken
    ? await postPublic<UsrLoginRep>('/usr/totp/confirm', {
        setup_token: setupToken,
        code,
      })
    : await apiFetch<UsrLoginRep>('/usr/totp/confirm', {
        method: 'POST',
        body: JSON.stringify({ setup_token: '', code }),
      })
  if (rep.jwt) {
    setSession({ jwt: rep.jwt, refresh_token: rep.refresh_token ?? '' })
  }
  return rep
}

/** Disable 2FA for the current user (requires a current code). */
export function disableTotp(code: string): Promise<void> {
  return apiFetch<void>('/usr/totp/disable', {
    method: 'POST',
    body: JSON.stringify({ code }),
  })
}

/** Admin-reset another user's 2FA (clears it so they can re-enrol). */
export function resetUserTotp(id: number | string): Promise<void> {
  return apiFetch<void>(`/usr/${encodeURIComponent(String(id))}/totp/reset`, {
    method: 'POST',
  })
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
