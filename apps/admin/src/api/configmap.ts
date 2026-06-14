import { apiFetch } from './http'
import { buildListQuery } from './query'
import type {
  ConfigMapCreateRep,
  ConfigMapCreateReq,
  ConfigMapListRep,
  ConfigMapListReq,
  ConfigMapMain,
  ConfigMapUpdateReq,
} from './types'

/**
 * REST client for the kusec `ConfigMap` service (see
 * api/proto/kusec_v1/configmap.proto). Routes are singular per the gateway
 * contract: `/configmap`, `/configmap/{id}`. A ConfigMap is scoped to an App
 * via `app_id` and is the non-sensitive sibling of a Secret.
 */

export function listConfigMaps(
  req: ConfigMapListReq = {},
): Promise<ConfigMapListRep> {
  const query = buildListQuery(req.list_params, {
    app_id: req.app_id,
    active: req.active,
    search: req.search,
  })
  return apiFetch<ConfigMapListRep>(`/configmap${query}`)
}

export function getConfigMap(id: string): Promise<ConfigMapMain> {
  return apiFetch<ConfigMapMain>(`/configmap/${encodeURIComponent(id)}`)
}

export function createConfigMap(
  req: ConfigMapCreateReq,
): Promise<ConfigMapCreateRep> {
  return apiFetch<ConfigMapCreateRep>('/configmap', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateConfigMap(
  id: string,
  req: ConfigMapUpdateReq,
): Promise<void> {
  return apiFetch<void>(`/configmap/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteConfigMap(id: string): Promise<void> {
  return apiFetch<void>(`/configmap/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
