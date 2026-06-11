import { apiFetch } from './http'
import { buildListQuery } from './query'
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

export function listApps(req: AppListReq = {}): Promise<AppListRep> {
  const query = buildListQuery(req.list_params, {
    active: req.active,
    namespace: req.namespace,
    search: req.search,
  })
  return apiFetch<AppListRep>(`/app${query}`)
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
