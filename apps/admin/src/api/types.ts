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
  /** Whether the user has 2FA (TOTP) enabled. */
  totp_enabled?: boolean
}

/**
 * `UsrLoginRep` — the issued token pair, or a 2FA continuation signal.
 *
 * On a fully successful login the pair (`jwt` + `refresh_token`) is set. When
 * 2FA stands between the user and a session, the password was accepted but no
 * tokens are issued yet — instead exactly one flag is set:
 *   - `totp_required`: re-submit the login with a `totp_code`;
 *   - `totp_setup_required`: the (admin) account must enable 2FA first;
 *     `setup_token` authorises the TOTP enroll/confirm endpoints.
 */
export interface UsrLoginRep {
  jwt: string
  refresh_token: string
  totp_required?: boolean
  totp_setup_required?: boolean
  setup_token?: string
}

/** `UsrEnrollTotpRep` — secret + otpauth URL to bind in an authenticator app. */
export interface UsrEnrollTotpRep {
  secret: string
  otpauth_url: string
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
  /**
   * When true, the k8s secret name equals `slug_name` (no prefix, no app-slug).
   * Changing this flag is admin-only (enforced by the backend).
   */
  exact_slug: boolean
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
  /** Name without prefix/app-slug. Setting true is admin-only. */
  exact_slug?: boolean
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
  /** Changing this flag is admin-only. */
  exact_slug?: boolean
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
  /** Config-map counter (sibling of `secret`). */
  configmap: DashboardCount
  /** Config-item counter (sibling of `item`). */
  config_item: DashboardCount
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
 * `KubeSyncConfigMapsReq` — sync scope for config maps. Same semantics as
 * `KubeSyncSecretsReq`: empty/omitted `app_id` syncs every accessible app.
 */
export interface KubeSyncConfigMapsReq {
  app_id?: string
}

/**
 * Config-map sync result (`KubeSyncConfigMapsRep`). Identical shape to
 * `KubeSyncSecretsRep`; lists are "namespace/name".
 */
export interface KubeSyncConfigMapsRep {
  created: string[]
  updated: string[]
  deleted: string[]
  unchanged: number | string
  errors: string[]
}

/** `KubeSyncReq` — combined secrets + config maps sync scope. */
export interface KubeSyncReq {
  app_id?: string
}

/**
 * Combined sync result (`KubeSyncRep`) — secrets and config maps reconciled
 * in a single call under one lock.
 */
export interface KubeSyncRep {
  secrets: KubeSyncSecretsRep
  configmaps: KubeSyncConfigMapsRep
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

/**
 * `KubeImportSecretReq` — import one cluster secret into `app_id`.
 * `secret_slug` is the landing kusec secret name (required). When a secret with
 * that slug already exists, missing keys are added and matching keys are
 * overridden with the cluster value.
 */
export interface KubeImportSecretReq {
  app_id: string
  namespace: string
  name: string
  secret_slug: string
}

/**
 * Cluster secret import result (`KubeImportSecretRep`).
 * `secret_created` is false when an existing secret was topped up.
 * `created_items`/`updated_items` are int64 → may arrive as string.
 */
export interface KubeImportSecretRep {
  secret_id: string
  secret_slug: string
  secret_created: boolean
  created_items: number | string
  updated_items: number | string
}

/**
 * One key/value pair from a live cluster object (`KubeClusterResourceItemSt`).
 * `encoding` is 'plain' for text or 'base64' for binary values.
 */
export interface KubeClusterResourceItemSt {
  key: string
  value: string
  encoding: string
}

/**
 * Live k8s object (secret or config map) read from the cluster for comparison
 * (`KubeClusterResourceRep`). `in_cluster` is false when the backend runs
 * outside a cluster; `found` is false when the object is not synced yet.
 */
export interface KubeClusterResourceRep {
  in_cluster: boolean
  found: boolean
  namespace: string
  name: string
  /** k8s secret type (empty for config maps and Opaque secrets). */
  type: string
  managed: boolean
  items: KubeClusterResourceItemSt[]
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
  /** Value size in bytes (computed server-side; `int64` → JSON string). */
  value_size: number | string
  /** Truncated HMAC fingerprint of the value (16 hex). */
  value_hash: string
}

/** `ItemListReq` — filters for the list endpoint. */
export interface ItemListReq {
  list_params?: ListParams
  secret_id?: string
  /** Fetch items for several secrets in one request (no pagination). */
  secret_ids?: string[]
  /** Fetch items of every secret of an application in one request. */
  app_id?: string
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

/**
 * TypeScript mirrors of the kusec `ConfigMap` proto messages
 * (see api/proto/kusec_v1/configmap.proto).
 *
 * A ConfigMap is the non-sensitive sibling of a Secret: it belongs to an App
 * via `app_id` (FK, ON DELETE CASCADE) and holds plain config items. Unlike a
 * Secret it has no k8s type (a k8s ConfigMap is always untyped).
 */

/** ConfigMap entity (`ConfigMapMain`). */
export interface ConfigMapMain {
  id: string
  created_at: string
  updated_at: string
  app_id: string
  active: boolean
  slug_name: string
  description: string
  /** Full name of the resulting k8s configmap; computed by the backend. */
  kube_configmap_name: string
  /**
   * When true, the k8s configmap name equals `slug_name` (no prefix, no
   * app-slug). Changing this flag is admin-only (enforced by the backend).
   */
  exact_slug: boolean
}

/** `ConfigMapListReq` — filters for the list endpoint. */
export interface ConfigMapListReq {
  list_params?: ListParams
  app_id?: string
  active?: boolean
  search?: string
}

/** `ConfigMapListRep`. */
export interface ConfigMapListRep {
  pagination_info?: PaginationInfo
  results: ConfigMapMain[]
}

/** `ConfigMapCreateReq`. */
export interface ConfigMapCreateReq {
  app_id: string
  active?: boolean
  slug_name: string
  description: string
  /** Name without prefix/app-slug. Setting true is admin-only. */
  exact_slug?: boolean
}

/** `ConfigMapCreateRep`. */
export interface ConfigMapCreateRep {
  id: string
}

/** `ConfigMapUpdateReq`. */
export interface ConfigMapUpdateReq {
  app_id?: string
  active?: boolean
  slug_name?: string
  description?: string
  /** Changing this flag is admin-only. */
  exact_slug?: boolean
}

/**
 * TypeScript mirrors of the kusec `ConfigItem` proto messages
 * (see api/proto/kusec_v1/configitem.proto).
 *
 * A ConfigItem belongs to a ConfigMap via `configmap_id` (FK, ON DELETE
 * CASCADE). `key` is unique per config map (`uq_config_item_configmap_id_key`).
 * Values are not sensitive, but share the Item value shape (format/encoding).
 */

/** ConfigItem entity (`ConfigItemMain`). */
export interface ConfigItemMain {
  id: string
  created_at: string
  updated_at: string
  configmap_id: string
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

/** `ConfigItemListReq` — filters for the list endpoint. */
export interface ConfigItemListReq {
  list_params?: ListParams
  configmap_id?: string
  /** Fetch items for several config maps in one request (no pagination). */
  configmap_ids?: string[]
  /** Fetch items of every config map of an application in one request. */
  app_id?: string
  active?: boolean
  search?: string
}

/** `ConfigItemListRep`. */
export interface ConfigItemListRep {
  pagination_info?: PaginationInfo
  results: ConfigItemMain[]
}

/** `ConfigItemCreateReq`. */
export interface ConfigItemCreateReq {
  configmap_id: string
  active?: boolean
  key: string
  value: string
  value_format?: ValueFormat
  encoding?: ValueEncoding
  file_name?: string
  content_type?: string
  description: string
}

/** `ConfigItemCreateRep`. */
export interface ConfigItemCreateRep {
  id: string
}

/** `ConfigItemUpdateReq`. */
export interface ConfigItemUpdateReq {
  configmap_id?: string
  active?: boolean
  key?: string
  value?: string
  value_format?: ValueFormat
  encoding?: ValueEncoding
  file_name?: string
  content_type?: string
  description?: string
}

// ── API keys ───────────────────────────────────────────────

/**
 * API key (`ApiKeyMain`) — long-lived machine credential (MCP agents, CI).
 * Only a hash is stored server-side; `key_prefix` identifies the key in lists.
 * Note: `usr_id` is an `int64` the gateway serialises as a JSON string.
 */
export interface ApiKeyMain {
  id: string
  created_at: string
  updated_at: string
  usr_id: number
  active: boolean
  /** Deprecated alias: `scope === 'mcp_only'`. */
  mcp_only: boolean
  /**
   * Access scope: `full` (owner's rights), `read_only` (whitelisted read
   * methods of the main API, values masked) or `mcp_only` (accepted only by
   * the embedded MCP endpoint).
   */
  scope: ApiKeyScope
  name: string
  key_prefix: string
  last_used_at: string | null
}

export type ApiKeyScope = 'full' | 'read_only' | 'mcp_only'

/** `ApiKeyListReq` — non-admins always get only their own keys. */
export interface ApiKeyListReq {
  list_params?: ListParams
  /** Admin-only filter by owner. */
  usr_id?: number | string
  active?: boolean
}

/** `ApiKeyListRep`. */
export interface ApiKeyListRep {
  pagination_info?: PaginationInfo
  results: ApiKeyMain[]
}

/** `ApiKeyCreateReq` — `usr_id` (admin-only) targets another user. */
export interface ApiKeyCreateReq {
  name: string
  usr_id?: number | string
  /** `full` and `read_only` are admin-only; defaults to `full`. */
  scope?: ApiKeyScope
}

/** `ApiKeyCreateRep` — `key` is revealed once and cannot be fetched again. */
export interface ApiKeyCreateRep {
  id: string
  key: string
}

/** `ApiKeyUpdateReq`. */
export interface ApiKeyUpdateReq {
  active?: boolean
  name?: string
  /** Non-admins may only narrow their own `full` key. */
  scope?: ApiKeyScope
}


// ── Audit ──────────────────────────────────────────────────

/**
 * One changed field of an audit entry. Non-secret fields carry `old`/`new`
 * verbatim (absent side = create/delete). Secret item values (and oversized
 * config values) carry only an HMAC fingerprint and byte size.
 */
export interface AuditChange {
  field: string
  old?: string | null
  new?: string | null
  old_hash?: string
  new_hash?: string
  /** `int64` → JSON string. */
  old_size?: number | string | null
  new_size?: number | string | null
  truncated?: boolean
}

export type AuditAction =
  | 'create'
  | 'update'
  | 'delete'
  | 'activate'
  | 'deactivate'
  | 'sync'
  | 'import'

/** `AuditMain` — one immutable audit entry (append-only feed). */
export interface AuditMain {
  id: number | string
  created_at: string
  actor_usr_id: number | string | null
  actor_api_key_id: string | null
  actor_name: string
  /** ui | api | mcp | system */
  source: string
  request_id: string
  /** app | secret | item | configmap | config_item | api_key | usr | sync_run */
  entity_type: string
  entity_id: string
  app_id: string | null
  namespace: string
  app_slug: string
  /** Secret | ConfigMap | "" */
  kube_kind: string
  kube_name: string
  key: string
  action: AuditAction
  changes: AuditChange[]
  batch_id: string | null
}

export interface AuditListReq {
  list_params?: ListParams
  app_id?: string
  namespace?: string
  app_slug?: string
  kube_name?: string
  entity_type?: string
  entity_id?: string
  action?: string
  actor_usr_id?: number | string
  key?: string
  batch_id?: string
  /** RFC3339 bounds: `created_at >= gte`, `< lt`. */
  created_at_gte?: string
  created_at_lt?: string
}

export interface AuditListRep {
  pagination_info?: PaginationInfo
  results: AuditMain[]
}

// ── Sync runs ──────────────────────────────────────────────

/** One k8s object touched by a sync run (key names only, no values). */
export interface SyncRunObject {
  namespace: string
  kube_kind: string
  kube_name: string
  /** created | updated | deleted | unchanged | error */
  op: string
  error: string
  content_hash: string
  changed_keys: string[]
}

/** `SyncRunMain` — one sync run; `objects` is filled only by Get. */
export interface SyncRunMain {
  id: string
  started_at: string
  finished_at: string | null
  /** running | ok | partial | error */
  status: string
  error: string
  actor_usr_id: number | string | null
  actor_api_key_id: string | null
  actor_name: string
  source: string
  request_id: string
  /** `null` — sync of all accessible apps. */
  app_id: string | null
  duration_ms: number | string
  objects: SyncRunObject[]
}

export interface SyncRunListReq {
  list_params?: ListParams
  app_id?: string
  /** Runs that touched objects in this namespace. */
  namespace?: string
  status?: string
  started_at_gte?: string
  started_at_lt?: string
}

export interface SyncRunListRep {
  pagination_info?: PaginationInfo
  results: SyncRunMain[]
}
