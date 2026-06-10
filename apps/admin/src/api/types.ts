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
}

/** `UsrLoginRep` — holds the issued JWT. */
export interface UsrLoginRep {
  jwt: string
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
}

/**
 * TypeScript mirrors of the kusec `Item` proto messages
 * (see api/proto/kusec_v1/item.proto).
 *
 * An Item belongs to a Secret via `secret_id` (FK, ON DELETE CASCADE).
 * `key` is unique per secret (`uq_item_secret_id_key`). `value` is sensitive.
 */

/** Item entity (`ItemMain`). */
export interface ItemMain {
  id: string
  created_at: string
  updated_at: string
  secret_id: string
  active: boolean
  key: string
  value: string
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
  description?: string
}
