import { getSecret, listSecrets } from '@/api/secret'
import type { SecretMain } from '@/api/types'
import { createOptionsLookup } from './createOptionsLookup'

/** Shared lookup for the Secret ↔ Item relationship. */
export const useSecretOptions = createOptionsLookup<SecretMain>({
  list: (query) =>
    listSecrets({
      list_params: { page: 0, page_size: 50 },
      ...(query ? { search: query } : {}),
    }).then((rep) => rep.results ?? []),
  get: getSecret,
  idOf: (secret) => secret.id,
  labelOf: (secret) => secret.slug_name,
})
