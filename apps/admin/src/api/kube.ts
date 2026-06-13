import { apiFetch } from './http'
import type { KubeListNamespacesRep, KubeSyncSecretsRep } from './types'

/**
 * REST client for the kusec `Kube` service (see api/proto/kusec_v1/kube.proto).
 * Syncs active apps/secrets/items into Kubernetes secrets. Works only when the
 * backend runs inside a cluster; the sync is scoped to the caller's accessible
 * apps (admins reach all of them).
 */

/**
 * Sync managed secrets into the cluster. Pass an `appId` to sync a single
 * application; omit it to sync every application the caller can access.
 */
export function syncKubeSecrets(appId?: string): Promise<KubeSyncSecretsRep> {
  return apiFetch<KubeSyncSecretsRep>('/kube/sync-secret', {
    method: 'POST',
    body: JSON.stringify(appId ? { app_id: appId } : {}),
  })
}

export function listKubeNamespaces(): Promise<KubeListNamespacesRep> {
  return apiFetch<KubeListNamespacesRep>('/kube/namespace')
}
