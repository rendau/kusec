import { computed, ref } from 'vue'
import type { SelectOption } from 'naive-ui'

import { listSecrets, getSecret } from '@/api/secret'
import type { SecretMain } from '@/api/types'

/**
 * Shared lookup for the Secret ↔ Item relationship.
 *
 * Items only carry `secret_id`, so the UI must resolve Secret names for display
 * and offer a Secret picker for filtering/creating. The cache is module-scoped
 * so every Item view/modal/drawer shares one set of fetched Secrets.
 */

const cache = ref(new Map<string, SecretMain>())

function cacheSecrets(secrets: SecretMain[]): void {
  const next = new Map(cache.value)
  for (const secret of secrets) next.set(secret.id, secret)
  cache.value = next
}

export function useSecretOptions() {
  const loading = ref(false)

  const options = computed<SelectOption[]>(() =>
    Array.from(cache.value.values()).map((secret) => ({
      label: secret.slug_name,
      value: secret.id,
    })),
  )

  /** Load the first page of Secrets (optionally filtered) into the cache. */
  async function search(query = ''): Promise<void> {
    loading.value = true
    try {
      const rep = await listSecrets({
        list_params: { page: 0, page_size: 50 },
        ...(query.trim() ? { search: query.trim() } : {}),
      })
      cacheSecrets(rep.results ?? [])
    } finally {
      loading.value = false
    }
  }

  /** Ensure a single Secret is cached (used to resolve names not yet loaded). */
  async function ensure(id: string): Promise<void> {
    if (!id || cache.value.has(id)) return
    try {
      const secret = await getSecret(id)
      cacheSecrets([secret])
    } catch {
      // Leave unresolved — nameOf falls back to the raw id.
    }
  }

  /** Resolve a Secret display name (slug), falling back to the raw id. */
  function nameOf(id: string): string {
    const secret = cache.value.get(id)
    return secret ? secret.slug_name : id
  }

  return { options, loading, search, ensure, nameOf }
}
