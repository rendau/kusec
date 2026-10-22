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

export function authorize(
  req: OAuthAuthorizeReq,
  approve: boolean,
): Promise<OAuthAuthorizeRep> {
  return apiFetch<OAuthAuthorizeRep>('/oauth/authorize', {
    method: 'POST',
    body: JSON.stringify({ ...req, approve }),
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
