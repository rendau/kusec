import { computed, shallowRef } from 'vue'
import type { SelectOption } from 'naive-ui'
import { ref } from 'vue'

interface OptionsLookupConfig<T> {
  /** Load the first page of entities (optionally filtered by `query`). */
  list: (query: string) => Promise<T[]>
  /** Load a single entity by id (used to resolve names not yet cached). */
  get: (id: string) => Promise<T>
  idOf: (item: T) => string
  labelOf: (item: T) => string
}

/**
 * Factory for "id → entity" lookups behind remote NSelect pickers.
 *
 * Children reference parents by id only (item → secret → app), so the UI
 * must resolve display names and offer pickers. The cache is shared across
 * every component using the same lookup (module scope of the factory call).
 */
export function createOptionsLookup<T>(config: OptionsLookupConfig<T>) {
  const cache = shallowRef(new Map<string, T>())

  function cacheItems(items: T[]): void {
    const next = new Map(cache.value)
    for (const item of items) next.set(config.idOf(item), item)
    cache.value = next
  }

  return function useOptionsLookup() {
    const loading = ref(false)

    const options = computed<SelectOption[]>(() =>
      Array.from(cache.value.values())
        .map((item) => ({
          label: config.labelOf(item),
          value: config.idOf(item),
        }))
        // Cache is a Map in insertion order, and `ensure()` appends already
        // assigned entities out of order — sort by label so the picker always
        // shows options alphabetically regardless of how they got cached.
        .sort((a, b) => a.label.localeCompare(b.label)),
    )

    /** Load the first page (optionally filtered) into the cache. */
    async function search(query = ''): Promise<void> {
      loading.value = true
      try {
        cacheItems(await config.list(query.trim()))
      } finally {
        loading.value = false
      }
    }

    /** Ensure a single entity is cached. */
    async function ensure(id: string): Promise<void> {
      if (!id || cache.value.has(id)) return
      try {
        cacheItems([await config.get(id)])
      } catch {
        // Leave unresolved — nameOf falls back to the raw id.
      }
    }

    /** Resolve a display name, falling back to the raw id. */
    function nameOf(id: string): string {
      const item = cache.value.get(id)
      return item ? config.labelOf(item) : id
    }

    return { options, loading, search, ensure, nameOf }
  }
}
