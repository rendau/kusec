import { apiFetch } from './http'
import { buildListQuery } from './query'
import type {
  KubeClusterResourceRep,
  KubeImportSecretRep,
  KubeListClusterSecretsRep,
  KubeListNamespacesRep,
  KubeSyncConfigMapsRep,
  KubeSyncRep,
  KubeSyncSecretsRep,
} from './types'

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

/**
 * Sync managed config maps into the cluster. Pass an `appId` to sync a single
 * application; omit it to sync every application the caller can access.
 */
export function syncKubeConfigMaps(appId?: string): Promise<KubeSyncConfigMapsRep> {
  return apiFetch<KubeSyncConfigMapsRep>('/kube/sync-configmap', {
    method: 'POST',
    body: JSON.stringify(appId ? { app_id: appId } : {}),
  })
}

/**
 * Sync both secrets and config maps into the cluster in a single call (one
 * lock, no interleaving). Pass an `appId` to scope to one application.
 */
export function syncKube(appId?: string): Promise<KubeSyncRep> {
  return apiFetch<KubeSyncRep>('/kube/sync', {
    method: 'POST',
    body: JSON.stringify(appId ? { app_id: appId } : {}),
  })
}

export function listKubeNamespaces(): Promise<KubeListNamespacesRep> {
  return apiFetch<KubeListNamespacesRep>('/kube/namespace')
}

/**
 * Read the live k8s secret backing kusec secret `secretId` for comparison.
 * Access is scoped to the caller's apps. `in_cluster` is false outside a
 * cluster; `found` is false when the secret has not been synced yet.
 */
export function getClusterSecret(secretId: string): Promise<KubeClusterResourceRep> {
  return apiFetch<KubeClusterResourceRep>(
    `/kube/secret/${encodeURIComponent(secretId)}/cluster`,
  )
}

/**
 * Read the live k8s config map backing kusec config map `configMapId` for
 * comparison. Same access and `in_cluster`/`found` semantics as
 * {@link getClusterSecret}.
 */
export function getClusterConfigMap(
  configMapId: string,
): Promise<KubeClusterResourceRep> {
  return apiFetch<KubeClusterResourceRep>(
    `/kube/configmap/${encodeURIComponent(configMapId)}/cluster`,
  )
}

/**
 * List cluster secrets available for import (admin only). Pass a `namespace`
 * to scope the listing; omit it to list every non-system namespace. Values
 * are never returned — only keys for preview.
 */
export function listClusterSecrets(
  namespace?: string,
): Promise<KubeListClusterSecretsRep> {
  const query = buildListQuery(undefined, { namespace })
  return apiFetch<KubeListClusterSecretsRep>(`/kube/cluster-secret${query}`)
}

/**
 * Import one cluster secret into application `appId` (admin only). The secret
 * becomes a kusec secret with one item per data key. `secretSlug` is the
 * landing secret name; pass an empty string to default to the cluster name.
 * The cluster source is left intact.
 */
export function importClusterSecret(
  appId: string,
  ref: { namespace: string; name: string },
  secretSlug: string,
): Promise<KubeImportSecretRep> {
  return apiFetch<KubeImportSecretRep>('/kube/import-secret', {
    method: 'POST',
    body: JSON.stringify({
      app_id: appId,
      namespace: ref.namespace,
      name: ref.name,
      secret_slug: secretSlug,
    }),
  })
}
