import { computed, ref } from 'vue'
import type { SelectOption } from 'naive-ui'

import { listApps, getApp } from '@/api/app'
import type { AppMain } from '@/api/types'

/**
 * Shared lookup for the App ↔ Secret relationship.
 *
 * Secrets only carry `app_id`, so the UI must resolve App names for display
 * and offer an App picker for filtering/creating. The cache is module-scoped
 * so every Secret view/modal/drawer shares one set of fetched Apps.
 */

const cache = ref(new Map<string, AppMain>())

function cacheApps(apps: AppMain[]): void {
  const next = new Map(cache.value)
  for (const app of apps) next.set(app.id, app)
  cache.value = next
}

/** Human-readable label for an App option/tag. */
function labelFor(app: AppMain): string {
  return app.namespace ? `${app.name} · ${app.namespace}` : app.name
}

export function useAppOptions() {
  const loading = ref(false)

  const options = computed<SelectOption[]>(() =>
    Array.from(cache.value.values()).map((app) => ({
      label: labelFor(app),
      value: app.id,
    })),
  )

  /** Load the first page of Apps (optionally filtered) into the cache. */
  async function search(query = ''): Promise<void> {
    loading.value = true
    try {
      const rep = await listApps({
        list_params: { page: 0, page_size: 50 },
        ...(query.trim() ? { search: query.trim() } : {}),
      })
      cacheApps(rep.results ?? [])
    } finally {
      loading.value = false
    }
  }

  /** Ensure a single App is cached (used to resolve names not yet loaded). */
  async function ensure(id: string): Promise<void> {
    if (!id || cache.value.has(id)) return
    try {
      const app = await getApp(id)
      cacheApps([app])
    } catch {
      // Leave unresolved — nameOf falls back to the raw id.
    }
  }

  /** Resolve an App display name, falling back to the raw id. */
  function nameOf(id: string): string {
    const app = cache.value.get(id)
    return app ? labelFor(app) : id
  }

  return { options, loading, search, ensure, nameOf }
}
