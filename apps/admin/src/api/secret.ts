import { apiFetch } from './http'
import type {
  SecretCreateRep,
  SecretCreateReq,
  SecretListRep,
  SecretListReq,
  SecretMain,
  SecretUpdateReq,
} from './types'

/**
 * REST client for the kusec `Secret` service (see api/proto/kusec_v1/secret.proto).
 * Routes are singular per the gateway contract: `/secret`, `/secret/{id}`.
 * A Secret is scoped to an App via `app_id`.
 */

/** Flatten the (possibly nested) list request into gateway query params. */
function buildListQuery(req: SecretListReq): string {
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
  if (req.app_id) params.set('app_id', req.app_id)
  if (req.active != null) params.set('active', String(req.active))
  if (req.search) params.set('search', req.search)

  const query = params.toString()
  return query ? `?${query}` : ''
}

export function listSecrets(req: SecretListReq = {}): Promise<SecretListRep> {
  return apiFetch<SecretListRep>(`/secret${buildListQuery(req)}`)
}

export function getSecret(id: string): Promise<SecretMain> {
  return apiFetch<SecretMain>(`/secret/${encodeURIComponent(id)}`)
}

export function createSecret(req: SecretCreateReq): Promise<SecretCreateRep> {
  return apiFetch<SecretCreateRep>('/secret', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateSecret(id: string, req: SecretUpdateReq): Promise<void> {
  return apiFetch<void>(`/secret/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(req),
  })
}

export function deleteSecret(id: string): Promise<void> {
  return apiFetch<void>(`/secret/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
