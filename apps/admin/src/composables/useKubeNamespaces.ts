import { computed, ref } from 'vue'
import type { SelectOption } from 'naive-ui'

import { listKubeNamespaces } from '@/api/kube'

/**
 * Cluster namespaces for the app form picker. Cached module-wide: namespaces
 * change rarely, one fetch per session is enough. Outside a cluster (or on
 * error) the list stays empty — the form falls back to free-text input with
 * "default" preselected.
 */

const namespaces = ref<string[]>([])
const inCluster = ref(false)
const loaded = ref(false)
const loading = ref(false)

export function useKubeNamespaces() {
  const options = computed<SelectOption[]>(() =>
    namespaces.value.map((name) => ({ label: name, value: name })),
  )

  async function ensure(): Promise<void> {
    if (loaded.value || loading.value) return
    loading.value = true
    try {
      const rep = await listKubeNamespaces()
      inCluster.value = rep.in_cluster
      namespaces.value = rep.namespaces ?? []
      loaded.value = true
    } catch {
      // Недоступно (нет прав / сервис вне кластера со старым бэком) —
      // остаёмся на свободном вводе.
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  return { options, inCluster, loading, ensure }
}
