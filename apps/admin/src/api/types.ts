/**
 * TypeScript mirrors of the kusec `Usr` proto messages used by the
 * authentication flow (see api/proto/kusec_v1/usr.proto).
 */

/** Error body returned by the gRPC-gateway (`common.ErrorRep`). */
export interface ErrorRep {
  code: string
  message: string
  fields?: Record<string, string>
}

/** Authenticated user / profile (`UsrMain`). */
export interface UsrMain {
  id: number
  active: boolean
  is_admin: boolean
  name: string
  username: string
  /**
   * App ids the user may access. An empty list means *all* applications
   * (the backend treats no scope as full access); admins always have all.
   */
  app_ids?: string[]
}

/** `UsrLoginRep` — the issued token pair (short-lived access + refresh). */
export interface UsrLoginRep {
  jwt: string
  refresh_token: string
}

/** `UsrRefreshTokenReq`. */
export interface UsrRefreshTokenReq {
  refresh_token: string
}

/** `UsrBootstrapStatusRep`. */
export interface UsrBootstrapStatusRep {
  can_create_first_admin: boolean
}

/** `UsrCreateReq` — used to bootstrap the first admin. */
export interface UsrCreateReq {
  active?: boolean
  is_admin: boolean
  name: string
  username: string
  password: string
  /** App access scope; empty/omitted grants access to all applications. */
  app_ids?: string[]
}

/** `UsrCreateRep`. */
export interface UsrCreateRep {
  id: number
}

/** `UsrUpdateProfileReq`. */
export interface UsrUpdateProfileReq {
  name?: string
  username?: string
  password?: string
}

/**
 * `UsrListReq` — admin user listing filters.
 * Note: `UsrMain.id` is an `int64` the gateway serialises as a JSON string.
 */
export interface UsrListReq {
  list_params?: ListParams
  active?: boolean
  is_admin?: boolean
  search?: string
}

/** `UsrListRep`. */
export interface UsrListRep {
  pagination_info?: PaginationInfo
  results: UsrMain[]
}

/** `UsrUpdateReq` — partial update; omit `password` to keep it unchanged. */
export interface UsrUpdateReq {
  active?: boolean
  is_admin?: boolean
  name?: string
  username?: string
  password?: string
  /** App access scope; empty list grants access to all applications. */
  app_ids?: string[]
}

/**
 * TypeScript mirrors of the kusec `App` proto messages
 * (see api/proto/kusec_v1/app.proto).
 *
 * Note: the gRPC-gateway serialises `int64` fields as strings and
 * `google.protobuf.Timestamp` as RFC 3339 strings.
 */

/** Shared list parameters (`common.ListParamsSt`). */
export interface ListParams {
  /** Zero-based page index — the first page is `0` (project convention). */
  page?: number
  page_size?: number
  with_total_count?: boolean
  only_count?: boolean
  sort_name?: string
  sort?: string[]
}

/** Pagination block returned by list endpoints (`common.PaginationInfoSt`). */
export interface PaginationInfo {
  page: number
  page_size: number
  total_count: number
}

/** Application entity (`AppMain`). */
export interface AppMain {
  id: string
  created_at: string
  updated_at: string
  active: boolean
  namespace: string
  name: string
  slug_name: string
  description: string
}

/** `AppListReq` — filters for the list endpoint. */
export interface AppListReq {
  list_params?: ListParams
  active?: boolean
  namespace?: string
  search?: string
}

/** `AppListRep`. */
export interface AppListRep {
  pagination_info?: PaginationInfo
  results: AppMain[]
}

/** `AppCreateReq`. */
export interface AppCreateReq {
  active?: boolean
  namespace: string
  name: string
  slug_name: string
  description: string
}

/** `AppCreateRep`. */
export interface AppCreateRep {
  id: string
}

/** `AppUpdateReq`. */
export interface AppUpdateReq {
  active?: boolean
  namespace?: string
  name?: string
  slug_name?: string
  description?: string
}

/**
 * TypeScript mirrors of the kusec `Secret` proto messages
 * (see api/proto/kusec_v1/secret.proto).
 *
 * A Secret belongs to an App via `app_id` (FK, ON DELETE CASCADE).
 * `slug_name` is unique per app (`uq_secret_app_id_slug_name`).
 */

/** Secret entity (`SecretMain`). */
export interface SecretMain {
  id: string
  created_at: string
  updated_at: string
  app_id: string
  active: boolean
  slug_name: string
  description: string
  /** Full name of the resulting k8s secret; computed by the backend. */
  kube_secret_name: string
  /** K8s secret type (empty = Opaque), e.g. kubernetes.io/basic-auth. */
  kube_type: string
}

/** `SecretListReq` — filters for the list endpoint. */
export interface SecretListReq {
  list_params?: ListParams
  app_id?: string
  active?: boolean
  search?: string
}

/** `SecretListRep`. */
export interface SecretListRep {
  pagination_info?: PaginationInfo
  results: SecretMain[]
}

/** `SecretCreateReq`. */
export interface SecretCreateReq {
  app_id: string
  active?: boolean
  slug_name: string
  description: string
  /** K8s secret type (empty = Opaque). */
  kube_type?: string
}

/** `SecretCreateRep`. */
export interface SecretCreateRep {
  id: string
}

/** `SecretUpdateReq`. */
export interface SecretUpdateReq {
  app_id?: string
  active?: boolean
  slug_name?: string
  description?: string
  /** K8s secret type (empty = Opaque). */
  kube_type?: string
}

/**
 * TypeScript mirrors of the kusec `Item` proto messages
 * (see api/proto/kusec_v1/item.proto).
 *
 * An Item belongs to a Secret via `secret_id` (FK, ON DELETE CASCADE).
 * `key` is unique per secret (`uq_item_secret_id_key`). `value` is sensitive.
 */

/** Editor format of an item value (what the UI sends; server stores a string). */
export type ValueFormat = 'text' | 'yaml' | 'json'

/** Storage encoding of an item value. */
export type ValueEncoding = 'plain' | 'base64'

/**
 * Dashboard summary (`DashboardRep`, see api/proto/kusec_v1/dashboard.proto).
 * Note: the gateway serialises `int64` counters as JSON strings.
 */

/** `DashboardCountSt` — entity counter (total / active). */
export interface DashboardCount {
  total: number | string
  active: number | string
}

/** `DashboardRecentSecretSt` — recently updated secret, enriched for display. */
export interface DashboardRecentSecret {
  id: string
  app_id: string
  app_name: string
  slug_name: string
  description: string
  active: boolean
  updated_at: string
  item_count: number | string
}

/** `DashboardRep`. */
export interface DashboardRep {
  app: DashboardCount
  secret: DashboardCount
  item: DashboardCount
  usr: DashboardCount
  recent_secrets: DashboardRecentSecret[]
}

/** `KubeListNamespacesRep` — cluster namespaces for the app form picker. */
export interface KubeListNamespacesRep {
  /** false — the service runs outside a cluster (namespaces is empty). */
  in_cluster: boolean
  namespaces: string[]
}

/**
 * `KubeSyncSecretsReq` — sync scope. An empty/omitted `app_id` syncs every
 * application the caller can access; a set `app_id` syncs just that one.
 */
export interface KubeSyncSecretsReq {
  app_id?: string
}

/**
 * Kubernetes sync (`KubeSyncSecretsRep`, see api/proto/kusec_v1/kube.proto).
 * Lists are "namespace/name"; `unchanged` is an int64 → JSON string.
 */
export interface KubeSyncSecretsRep {
  created: string[]
  updated: string[]
  deleted: string[]
  unchanged: number | string
  errors: string[]
}

/**
 * `KubeClusterSecretSt` — one cluster secret offered for import. Values are
 * never sent; only `keys` (sorted) are listed for preview.
 */
export interface KubeClusterSecretSt {
  namespace: string
  name: string
  /** K8s secret type (empty = Opaque). */
  type: string
  keys: string[]
  /** Already managed by kusec (label managed-by=kusec). */
  managed: boolean
}

/**
 * `KubeListClusterSecretsRep` — cluster secrets available for import.
 * `in_cluster` is false when the service runs outside a cluster (empty list).
 */
export interface KubeListClusterSecretsRep {
  in_cluster: boolean
  secrets: KubeClusterSecretSt[]
}

/** `KubeImportSecretRefSt` — reference to a cluster secret to import. */
export interface KubeImportSecretRefSt {
  namespace: string
  name: string
}

/** `KubeImportSecretsReq` — import the selected secrets into `app_id`. */
export interface KubeImportSecretsReq {
  app_id: string
  secrets: KubeImportSecretRefSt[]
}

/**
 * Cluster secret import result (`KubeImportSecretsRep`).
 * `imported`/`skipped` lists are "namespace/name"; counters are int64 → string.
 */
export interface KubeImportSecretsRep {
  imported: string[]
  skipped: string[]
  errors: string[]
  created_secrets: number | string
  created_items: number | string
}

/** Item entity (`ItemMain`). */
export interface ItemMain {
  id: string
  created_at: string
  updated_at: string
  secret_id: string
  active: boolean
  key: string
  value: string
  /** Editor format of `value` (e.g. "text" | "yaml" | "json"). */
  value_format: string
  /** How `value` is stored: "plain" | "base64" (binary/file). */
  encoding: string
  /** Original file name when the value was uploaded as a file. */
  file_name: string
  /** MIME type of the uploaded file. */
  content_type: string
  description: string
}

/** `ItemListReq` — filters for the list endpoint. */
export interface ItemListReq {
  list_params?: ListParams
  secret_id?: string
  active?: boolean
  search?: string
}

/** `ItemListRep`. */
export interface ItemListRep {
  pagination_info?: PaginationInfo
  results: ItemMain[]
}

/** `ItemCreateReq`. */
export interface ItemCreateReq {
  secret_id: string
  active?: boolean
  key: string
  value: string
  value_format?: ValueFormat
  encoding?: ValueEncoding
  file_name?: string
  content_type?: string
  description: string
}

/** `ItemCreateRep`. */
export interface ItemCreateRep {
  id: string
}

/** `ItemUpdateReq`. */
export interface ItemUpdateReq {
  secret_id?: string
  active?: boolean
  key?: string
  value?: string
  value_format?: ValueFormat
  encoding?: ValueEncoding
  file_name?: string
  content_type?: string
  description?: string
}
