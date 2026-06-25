import { getApp, listApps } from '@/api/app'
import type { AppMain } from '@/api/types'
import { createOptionsLookup } from './createOptionsLookup'

/** Human-readable label for an App option/tag. */
function labelFor(app: AppMain): string {
  return app.namespace ? `${app.name} · ${app.namespace}` : app.name
}

/** Shared lookup for the App ↔ Secret relationship. */
export const useAppOptions = createOptionsLookup<AppMain>({
  list: (query) =>
    listApps({
      // page_size: 0 — без лимита (mobone не ставит LIMIT при PageSize == 0),
      // грузим все приложения; бэкенд отдаёт их отсортированными по имени.
      list_params: { page: 0, page_size: 0 },
      ...(query ? { search: query } : {}),
    }).then((rep) => rep.results ?? []),
  get: getApp,
  idOf: (app) => app.id,
  labelOf: labelFor,
})
