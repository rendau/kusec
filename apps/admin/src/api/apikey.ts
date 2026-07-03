import { apiFetch } from './http'
import { buildListQuery } from './query'
import type {
  ApiKeyCreateRep,
  ApiKeyCreateReq,
  ApiKeyListRep,
  ApiKeyListReq,
  ApiKeyUpdateReq,
} from './types'

/**
 * API keys (`/api-key`) — long-lived machine credentials. Non-admins manage
 * only their own keys (the backend forces the owner filter); admins may list
 * and issue keys for any user.
 */

export function listApiKeys(req: ApiKeyListReq = {}): Promise<ApiKeyListRep> {
  const query = buildListQuery(req.list_params, {
    usr_id: req.usr_id,
    active: req.active,
  })
  return apiFetch<ApiKeyListRep>(`/api-key${query}`)
}

/** The response `key` is shown once and cannot be retrieved again. */
export function createApiKey(req: ApiKeyCreateReq): Promise<ApiKeyCreateRep> {
  return apiFetch<ApiKeyCreateRep>('/api-key', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateApiKey(id: string, req: ApiKeyUpdateReq): Promise<void> {
  return apiFetch<void>(`/api-key/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteApiKey(id: string): Promise<void> {
  return apiFetch<void>(`/api-key/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
