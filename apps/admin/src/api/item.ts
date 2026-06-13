import { apiFetch } from './http'
import { buildListQuery } from './query'
import type {
  ItemCreateRep,
  ItemCreateReq,
  ItemListRep,
  ItemListReq,
  ItemMain,
  ItemUpdateReq,
} from './types'

/**
 * REST client for the kusec `Item` service (see api/proto/kusec_v1/item.proto).
 * Routes are singular per the gateway contract: `/item`, `/item/{id}`.
 * An Item is scoped to a Secret via `secret_id`.
 */

export function listItems(req: ItemListReq = {}): Promise<ItemListRep> {
  const query = buildListQuery(req.list_params, {
    secret_id: req.secret_id,
    secret_ids: req.secret_ids,
    active: req.active,
    search: req.search,
  })
  return apiFetch<ItemListRep>(`/item${query}`)
}

export function getItem(id: string): Promise<ItemMain> {
  return apiFetch<ItemMain>(`/item/${encodeURIComponent(id)}`)
}

export function createItem(req: ItemCreateReq): Promise<ItemCreateRep> {
  return apiFetch<ItemCreateRep>('/item', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateItem(id: string, req: ItemUpdateReq): Promise<void> {
  return apiFetch<void>(`/item/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteItem(id: string): Promise<void> {
  return apiFetch<void>(`/item/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
