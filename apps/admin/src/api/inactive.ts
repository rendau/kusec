import { apiFetch } from './http'
import { buildListQuery } from './query'
import type { InactiveDeleteReq, InactiveListRep } from './types'

/**
 * REST client for the kusec `Inactive` service
 * (see api/proto/kusec_v1/inactive.proto). Admin-only overview of deactivated
 * applications, secrets, config maps and their items.
 */

export function listInactive(appId?: string | null): Promise<InactiveListRep> {
  return apiFetch<InactiveListRep>(`/inactive${buildListQuery(undefined, { app_id: appId })}`)
}

export function deleteInactive(req: InactiveDeleteReq): Promise<void> {
  return apiFetch<void>('/inactive/delete', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}
