import { apiFetch } from './http'
import { buildListQuery } from './query'
import type {
  ConfigItemCreateRep,
  ConfigItemCreateReq,
  ConfigItemListRep,
  ConfigItemListReq,
  ConfigItemMain,
  ConfigItemUpdateReq,
} from './types'

/**
 * REST client for the kusec `ConfigItem` service (see
 * api/proto/kusec_v1/configitem.proto). Routes are singular per the gateway
 * contract: `/config-item`, `/config-item/{id}`. A ConfigItem is scoped to a
 * ConfigMap via `configmap_id`.
 */

export function listConfigItems(
  req: ConfigItemListReq = {},
): Promise<ConfigItemListRep> {
  const query = buildListQuery(req.list_params, {
    configmap_id: req.configmap_id,
    configmap_ids: req.configmap_ids,
    active: req.active,
    search: req.search,
  })
  return apiFetch<ConfigItemListRep>(`/config-item${query}`)
}

export function getConfigItem(id: string): Promise<ConfigItemMain> {
  return apiFetch<ConfigItemMain>(`/config-item/${encodeURIComponent(id)}`)
}

export function createConfigItem(
  req: ConfigItemCreateReq,
): Promise<ConfigItemCreateRep> {
  return apiFetch<ConfigItemCreateRep>('/config-item', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateConfigItem(
  id: string,
  req: ConfigItemUpdateReq,
): Promise<void> {
  return apiFetch<void>(`/config-item/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteConfigItem(id: string): Promise<void> {
  return apiFetch<void>(`/config-item/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
