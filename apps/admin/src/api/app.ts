import { apiFetch } from './http'
import type {
  AppCreateReq,
  AppCreateRep,
  AppListRep,
  AppListReq,
  AppMain,
  AppUpdateReq,
} from './types'

/**
 * REST client for the kusec `App` service (see api/proto/kusec_v1/app.proto).
 * Routes are singular per the gateway contract: `/app`, `/app/{id}`.
 */

/** Flatten the (possibly nested) list request into gateway query params. */
function buildListQuery(req: AppListReq): string {
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
  if (req.active != null) params.set('active', String(req.active))
  if (req.namespace) params.set('namespace', req.namespace)
  if (req.search) params.set('search', req.search)

  const query = params.toString()
  return query ? `?${query}` : ''
}

export function listApps(req: AppListReq = {}): Promise<AppListRep> {
  return apiFetch<AppListRep>(`/app${buildListQuery(req)}`)
}

export function getApp(id: string): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${encodeURIComponent(id)}`)
}

export function createApp(req: AppCreateReq): Promise<AppCreateRep> {
  return apiFetch<AppCreateRep>('/app', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export function updateApp(id: string, req: AppUpdateReq): Promise<void> {
  return apiFetch<void>(`/app/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export function deleteApp(id: string): Promise<void> {
  return apiFetch<void>(`/app/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
