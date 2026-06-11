import { ref, shallowRef, watch } from 'vue'
import { useMessage } from 'naive-ui'

import { apiErrorMessage } from '@/api/http'

interface UseDrawerResourceOptions<T, Id> {
  /** Drawer visibility (the `show` prop). */
  show: () => boolean
  /** Id of the resource to display; refetched when it changes. */
  id: () => Id | null
  fetch: (id: Id) => Promise<T>
  /** Optional post-load hook (e.g. resolve related names). */
  onLoaded?: (item: T) => void | Promise<void>
  /** Called when loading fails (close the drawer). */
  onError: () => void
}

/**
 * Shared fetch lifecycle of a detail drawer: load the resource whenever the
 * drawer opens or the id changes, report failures via toast and close.
 */
export function useDrawerResource<T, Id extends string | number>(
  options: UseDrawerResourceOptions<T, Id>,
) {
  const message = useMessage()

  const loading = ref(false)
  const item = shallowRef<T | null>(null)

  watch(
    () => [options.show(), options.id()] as const,
    async ([show, id]) => {
      if (!show || id == null || id === '') return
      loading.value = true
      item.value = null
      try {
        const loaded = await options.fetch(id)
        item.value = loaded
        await options.onLoaded?.(loaded)
      } catch (error) {
        message.error(apiErrorMessage(error, 'Failed to load'))
        options.onError()
      } finally {
        loading.value = false
      }
    },
  )

  return { loading, item }
}
