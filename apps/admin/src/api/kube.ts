import { apiFetch } from './http'
import type { KubeListNamespacesRep, KubeSyncSecretsRep } from './types'

/**
 * REST client for the kusec `Kube` service (see api/proto/kusec_v1/kube.proto).
 * Syncs active apps/secrets/items into Kubernetes secrets. Admin-only;
 * works only when the backend runs inside a cluster.
 */

export function syncKubeSecrets(): Promise<KubeSyncSecretsRep> {
  return apiFetch<KubeSyncSecretsRep>('/kube/sync-secret', {
    method: 'POST',
    body: '{}',
  })
}

export function listKubeNamespaces(): Promise<KubeListNamespacesRep> {
  return apiFetch<KubeListNamespacesRep>('/kube/namespace')
}
