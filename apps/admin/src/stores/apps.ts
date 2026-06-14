import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { listApps } from '@/api/app'
import type { AppMain } from '@/api/types'

/**
 * Applications navigation state, shared between the sidebar (which lists and
 * manages apps) and the workspace view (which reads the selected app). Keeping
 * the list here lets a rename/delete in the sidebar reflect everywhere at once.
 */
export const useAppsStore = defineStore('apps', () => {
  const apps = ref<AppMain[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  const count = computed(() => apps.value.length)

  async function load(): Promise<void> {
    loading.value = true
    try {
      // Apps are few; fetch them all without pagination.
      const rep = await listApps()
      apps.value = rep.results ?? []
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  async function ensureLoaded(): Promise<void> {
    if (!loaded.value) await load()
  }

  function getById(id: string): AppMain | null {
    return apps.value.find((app) => app.id === id) ?? null
  }

  /** Drop the cached list so the next login reloads it for the new session. */
  function reset(): void {
    apps.value = []
    loaded.value = false
    loading.value = false
  }

  return { apps, loading, loaded, count, load, refresh: load, ensureLoaded, getById, reset }
})
