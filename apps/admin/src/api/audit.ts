import { apiFetch } from './http'
import { buildListQuery } from './query'
import type { AuditListRep, AuditListReq } from './types'

/**
 * Audit log (`/audit`) — append-only feed of every mutation. Read-only:
 * entries are written server-side in the same transaction as the change and
 * cleaned up only by the background retention job.
 */

export function listAudit(req: AuditListReq = {}): Promise<AuditListRep> {
  const query = buildListQuery(req.list_params, {
    app_id: req.app_id,
    namespace: req.namespace,
    app_slug: req.app_slug,
    kube_name: req.kube_name,
    entity_type: req.entity_type,
    entity_id: req.entity_id,
    action: req.action,
    actor_usr_id: req.actor_usr_id,
    key: req.key,
    batch_id: req.batch_id,
    created_at_gte: req.created_at_gte,
    created_at_lt: req.created_at_lt,
  })
  return apiFetch<AuditListRep>(`/audit${query}`)
}
