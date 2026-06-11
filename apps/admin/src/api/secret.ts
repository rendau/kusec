import { apiFetch } from './http'
import { buildListQuery } from './query'
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

export function listSecrets(req: SecretListReq = {}): Promise<SecretListRep> {
  const query = buildListQuery(req.list_params, {
    app_id: req.app_id,
    active: req.active,
    search: req.search,
  })
  return apiFetch<SecretListRep>(`/secret${query}`)
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
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteSecret(id: string): Promise<void> {
  return apiFetch<void>(`/secret/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
