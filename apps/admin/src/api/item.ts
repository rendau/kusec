import { apiFetch } from './http'
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

/** Flatten the (possibly nested) list request into gateway query params. */
function buildListQuery(req: ItemListReq): string {
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
  if (req.secret_id) params.set('secret_id', req.secret_id)
  if (req.active != null) params.set('active', String(req.active))
  if (req.search) params.set('search', req.search)

  const query = params.toString()
  return query ? `?${query}` : ''
}

export function listItems(req: ItemListReq = {}): Promise<ItemListRep> {
  return apiFetch<ItemListRep>(`/item${buildListQuery(req)}`)
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
