import { ref } from 'vue'
import type { InjectionKey, Ref } from 'vue'

import { listConfigItems } from '@/api/configitem'
import type { ConfigItemMain } from '@/api/types'

/**
 * Shared config-items cache for the app workspace, provided by the ConfigMaps
 * section and consumed by every ConfigMapItemsPanel. Mirrors useSecretItems:
 * `prefetch` loads many config maps' items in a single
 * `/config-item?configmap_ids=...` call, while `ensure` lazily loads a single
 * config map only when the cache misses (manual expand). Panels read items
 * reactively from `itemsByConfigMap`.
 */
export interface ConfigItemsStore {
  /** configmapId → its items (`undefined` = not loaded yet). */
  itemsByConfigMap: Ref<Record<string, ConfigItemMain[] | undefined>>
  /** configmapId → a request is in flight. */
  loadingByConfigMap: Ref<Record<string, boolean>>
  /** Load several config maps' items in one request (skips cached/loading). */
  prefetch: (configmapIds: string[]) => Promise<void>
  /** Load a single config map's items if not cached/loading (lazy expand). */
  ensure: (configmapId: string) => Promise<void>
  /** Force a refetch of a single config map's items (after a mutation). */
  reload: (configmapId: string) => Promise<void>
  /** Drop the whole cache (e.g. when switching applications). */
  reset: () => void
}

export const configItemsKey: InjectionKey<ConfigItemsStore> =
  Symbol('configItems')

export function createConfigItemsStore(): ConfigItemsStore {
  const itemsByConfigMap = ref<Record<string, ConfigItemMain[] | undefined>>({})
  const loadingByConfigMap = ref<Record<string, boolean>>({})

  function setLoading(ids: string[], value: boolean): void {
    const next = { ...loadingByConfigMap.value }
    for (const id of ids) {
      if (value) next[id] = true
      else delete next[id]
    }
    loadingByConfigMap.value = next
  }

  function setItems(entries: Record<string, ConfigItemMain[]>): void {
    itemsByConfigMap.value = { ...itemsByConfigMap.value, ...entries }
  }

  async function prefetch(configmapIds: string[]): Promise<void> {
    const pending = configmapIds.filter(
      (id) =>
        itemsByConfigMap.value[id] === undefined &&
        !loadingByConfigMap.value[id],
    )
    if (!pending.length) return

    setLoading(pending, true)
    try {
      const rep = await listConfigItems({ configmap_ids: pending })
      // Seed empty arrays so config maps with no items count as loaded.
      const grouped: Record<string, ConfigItemMain[]> = {}
      for (const id of pending) grouped[id] = []
      for (const item of rep.results ?? []) {
        ;(grouped[item.configmap_id] ??= []).push(item)
      }
      setItems(grouped)
    } finally {
      setLoading(pending, false)
    }
  }

  async function fetchOne(configmapId: string): Promise<void> {
    setLoading([configmapId], true)
    try {
      const rep = await listConfigItems({ configmap_id: configmapId })
      setItems({ [configmapId]: rep.results ?? [] })
    } finally {
      setLoading([configmapId], false)
    }
  }

  async function ensure(configmapId: string): Promise<void> {
    if (
      itemsByConfigMap.value[configmapId] !== undefined ||
      loadingByConfigMap.value[configmapId]
    ) {
      return
    }
    await fetchOne(configmapId)
  }

  function reset(): void {
    itemsByConfigMap.value = {}
    loadingByConfigMap.value = {}
  }

  return {
    itemsByConfigMap,
    loadingByConfigMap,
    prefetch,
    ensure,
    reload: fetchOne,
    reset,
  }
}
