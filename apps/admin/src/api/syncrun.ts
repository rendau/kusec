import { apiFetch } from './http'
import { buildListQuery } from './query'
import type { SyncRunListRep, SyncRunListReq, SyncRunMain } from './types'

/**
 * Sync journal (`/sync-run`) — read-only history of kusec → Kubernetes sync
 * runs. `getSyncRun` also returns the touched k8s objects (key names only).
 */

export function listSyncRuns(req: SyncRunListReq = {}): Promise<SyncRunListRep> {
  const query = buildListQuery(req.list_params, {
    app_id: req.app_id,
    namespace: req.namespace,
    status: req.status,
    started_at_gte: req.started_at_gte,
    started_at_lt: req.started_at_lt,
  })
  return apiFetch<SyncRunListRep>(`/sync-run${query}`)
}

export function getSyncRun(id: string): Promise<SyncRunMain> {
  return apiFetch<SyncRunMain>(`/sync-run/${encodeURIComponent(id)}`)
}
