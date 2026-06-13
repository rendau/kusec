import type { ListParams } from './types'

/**
 * Flatten a list request (shared `list_params` + entity-specific filters)
 * into gRPC-gateway query params: `?list_params.page=0&active=true&...`.
 *
 * Filter values that are `undefined`, `null` or `''` are omitted; everything
 * else (including `false` and `0`) is serialised with `String()`.
 */
export function buildListQuery(
  listParams: ListParams | undefined,
  filters: Record<
    string,
    string | number | boolean | string[] | null | undefined
  > = {},
): string {
  const params = new URLSearchParams()

  if (listParams) {
    if (listParams.page != null) params.set('list_params.page', String(listParams.page))
    if (listParams.page_size != null)
      params.set('list_params.page_size', String(listParams.page_size))
    if (listParams.with_total_count != null)
      params.set('list_params.with_total_count', String(listParams.with_total_count))
    if (listParams.only_count != null)
      params.set('list_params.only_count', String(listParams.only_count))
    if (listParams.sort_name) params.set('list_params.sort_name', listParams.sort_name)
    for (const s of listParams.sort ?? []) params.append('list_params.sort', s)
  }

  for (const [key, value] of Object.entries(filters)) {
    if (value == null || value === '') continue
    // Repeated query params (e.g. `secret_ids=a&secret_ids=b`) for list fields.
    if (Array.isArray(value)) {
      for (const v of value) params.append(key, String(v))
      continue
    }
    params.set(key, String(value))
  }

  const query = params.toString()
  return query ? `?${query}` : ''
}
