import { ref } from 'vue'
import type { InjectionKey, Ref } from 'vue'

import { listItems } from '@/api/item'
import type { ItemMain } from '@/api/types'

/**
 * Shared items cache for the app workspace, provided by AppWorkspaceView and
 * consumed by every SecretItemsPanel. The point is to avoid one request per
 * secret when several panels expand at once: `prefetch` loads many secrets in a
 * single `/item?secret_ids=...` call, while `ensure` lazily loads a single
 * secret only when the cache misses (manual expand). Panels read items
 * reactively from `itemsBySecret`.
 */
export interface SecretItemsStore {
  /** secretId → its items (`undefined` = not loaded yet). */
  itemsBySecret: Ref<Record<string, ItemMain[] | undefined>>
  /** secretId → a request is in flight. */
  loadingBySecret: Ref<Record<string, boolean>>
  /** Load several secrets' items in one request (skips already cached/loading). */
  prefetch: (secretIds: string[]) => Promise<void>
  /** Load a single secret's items if not cached/loading (lazy expand). */
  ensure: (secretId: string) => Promise<void>
  /** Force a refetch of a single secret's items (after a mutation). */
  reload: (secretId: string) => Promise<void>
  /** Drop the whole cache (e.g. when switching applications). */
  reset: () => void
}

export const secretItemsKey: InjectionKey<SecretItemsStore> = Symbol('secretItems')

export function createSecretItemsStore(): SecretItemsStore {
  const itemsBySecret = ref<Record<string, ItemMain[] | undefined>>({})
  const loadingBySecret = ref<Record<string, boolean>>({})

  function setLoading(ids: string[], value: boolean): void {
    const next = { ...loadingBySecret.value }
    for (const id of ids) {
      if (value) next[id] = true
      else delete next[id]
    }
    loadingBySecret.value = next
  }

  function setItems(entries: Record<string, ItemMain[]>): void {
    itemsBySecret.value = { ...itemsBySecret.value, ...entries }
  }

  async function prefetch(secretIds: string[]): Promise<void> {
    const pending = secretIds.filter(
      (id) => itemsBySecret.value[id] === undefined && !loadingBySecret.value[id],
    )
    if (!pending.length) return

    setLoading(pending, true)
    try {
      const rep = await listItems({ secret_ids: pending })
      // Seed empty arrays so secrets with no items count as loaded.
      const grouped: Record<string, ItemMain[]> = {}
      for (const id of pending) grouped[id] = []
      for (const item of rep.results ?? []) {
        ;(grouped[item.secret_id] ??= []).push(item)
      }
      setItems(grouped)
    } finally {
      setLoading(pending, false)
    }
  }

  async function fetchOne(secretId: string): Promise<void> {
    setLoading([secretId], true)
    try {
      const rep = await listItems({ secret_id: secretId })
      setItems({ [secretId]: rep.results ?? [] })
    } finally {
      setLoading([secretId], false)
    }
  }

  async function ensure(secretId: string): Promise<void> {
    if (
      itemsBySecret.value[secretId] !== undefined ||
      loadingBySecret.value[secretId]
    ) {
      return
    }
    await fetchOne(secretId)
  }

  function reset(): void {
    itemsBySecret.value = {}
    loadingBySecret.value = {}
  }

  return { itemsBySecret, loadingBySecret, prefetch, ensure, reload: fetchOne, reset }
}
