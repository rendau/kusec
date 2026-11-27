import type { UsrMain } from '@/api/types'

/**
 * Which applications a user can open — the single place that reads
 * `is_admin` / `all_apps` / `app_ids`, so every screen shows the same three
 * states:
 *   - `all`    — an admin, or a non-admin with `all_apps`;
 *   - `listed` — only the applications in `app_ids`;
 *   - `none`   — `all_apps` is off and the list is empty: no access at all.
 *
 * An empty `app_ids` is never a shortcut for "all".
 */
export type UsrAccessKind = 'all' | 'listed' | 'none'

type AccessFields = Pick<UsrMain, 'is_admin' | 'all_apps' | 'app_ids'>

export function usrAccessKind(user: AccessFields): UsrAccessKind {
  if (user.is_admin || user.all_apps) return 'all'
  return user.app_ids?.length ? 'listed' : 'none'
}

/** Short label for tags in lists: "All apps" / "3 apps" / "No access". */
export function usrAccessLabel(user: AccessFields): string {
  switch (usrAccessKind(user)) {
    case 'all':
      return 'All apps'
    case 'none':
      return 'No access'
    case 'listed': {
      const count = user.app_ids?.length ?? 0
      return `${count} app${count === 1 ? '' : 's'}`
    }
  }
}
