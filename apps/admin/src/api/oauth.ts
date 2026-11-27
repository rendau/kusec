import { apiFetch } from './http'
import type {
  OAuthAuthorizeInfoRep,
  OAuthAuthorizeRep,
  OAuthAuthorizeReq,
  OAuthGrantListRep,
  OAuthGrantMain,
} from './types'

/**
 * OAuth sign-in of the embedded MCP server: the consent page and the
 * connected clients of the current user. Works only from an admin UI login.
 */

// int64 fields arrive as strings from grpc-gateway.
function normalizeGrant(value: OAuthGrantMain): OAuthGrantMain {
  return {
    id: value?.id || '',
    client_name: value?.client_name || '',
    redirect_host: value?.redirect_host || '',
    created_at_unix: Number(value?.created_at_unix || 0),
    connected_at_unix: Number(value?.connected_at_unix || 0),
    last_used_at_unix: Number(value?.last_used_at_unix || 0),
  }
}

export function describeAuthorization(
  req: OAuthAuthorizeReq,
): Promise<OAuthAuthorizeInfoRep> {
  return apiFetch<OAuthAuthorizeInfoRep>('/oauth/authorize/describe', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

/**
 * Record the decision. Allowing needs the current password (the connection
 * outlives sign-out and a password change): a wrong one is rejected with
 * `current_password_invalid`, repeated failures — `too_many_attempts`.
 * Denying needs none.
 */
export function authorize(
  req: OAuthAuthorizeReq,
  approve: boolean,
  currentPassword = '',
): Promise<OAuthAuthorizeRep> {
  return apiFetch<OAuthAuthorizeRep>('/oauth/authorize', {
    method: 'POST',
    body: JSON.stringify({
      ...req,
      approve,
      ...(approve ? { current_password: currentPassword } : {}),
    }),
  })
}

export function listOAuthGrants(): Promise<OAuthGrantListRep> {
  return apiFetch<OAuthGrantListRep>('/oauth/grant').then((rep) => ({
    results: (rep.results || []).map(normalizeGrant),
  }))
}

export function deleteOAuthGrant(id: string): Promise<void> {
  return apiFetch<void>(`/oauth/grant/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
