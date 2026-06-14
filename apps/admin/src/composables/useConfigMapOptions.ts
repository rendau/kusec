import { getConfigMap, listConfigMaps } from '@/api/configmap'
import type { ConfigMapMain } from '@/api/types'
import { createOptionsLookup } from './createOptionsLookup'

/** Shared lookup for the ConfigMap ↔ ConfigItem relationship. */
export const useConfigMapOptions = createOptionsLookup<ConfigMapMain>({
  list: (query) =>
    listConfigMaps({
      list_params: { page: 0, page_size: 50 },
      ...(query ? { search: query } : {}),
    }).then((rep) => rep.results ?? []),
  get: getConfigMap,
  idOf: (configmap) => configmap.id,
  labelOf: (configmap) => configmap.slug_name,
})
