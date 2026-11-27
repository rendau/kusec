import { h, ref } from 'vue'
import { useDialog, useNotification } from 'naive-ui'

import { apiErrorRequestId } from '@/api/errors'
import { ApiError, apiErrorMessage } from '@/api/http'
import { syncKube } from '@/api/kube'
import type { KubeSyncConfigMapsRep, KubeSyncSecretsRep } from '@/api/types'

/**
 * Sync-specific wording for error codes; the rest (`not_in_cluster`,
 * `sync_in_progress`, …) reads the same as anywhere via `apiErrorMessage`.
 */
const SYNC_ERROR_HINTS: Record<string, string> = {
  no_permission: 'You do not have access to the requested application',
  // Set by the API layer when no answer came from kusec itself (proxy timeout).
  service_not_available:
    'No response received in time — the deploy is likely still running on the server',
}

function syncErrorText(error: unknown): string {
  const fallback = 'Kubernetes sync failed'
  if (!(error instanceof ApiError)) return fallback
  // A request id means kusec did answer — with an unexpected failure of its
  // own, so the "no response in time" hint would point the wrong way.
  if (apiErrorRequestId(error)) return apiErrorMessage(error, fallback)
  return SYNC_ERROR_HINTS[error.code] ?? apiErrorMessage(error, fallback)
}

interface SyncOptions {
  /** Sync a single application; omit to sync every accessible application. */
  appId?: string
  /** App name for the confirmation copy (only relevant with `appId`). */
  appName?: string
}

/** Finished-sync report shown in `KubeSyncReportModal`. */
export interface KubeSyncReportSt {
  /** Human-readable sync scope ("all accessible applications" / one app). */
  scope: string
  finishedAt: Date
  secrets: KubeSyncSecretsRep
  configmaps: KubeSyncConfigMapsRep
}

// Shared across every useKubeSync() call site: the sync report is a global
// concern, and the modal that renders it is mounted once in DefaultLayout.
const report = ref<KubeSyncReportSt | null>(null)
const reportVisible = ref(false)

/**
 * Shared "sync to Kubernetes" flow used by the dashboard (all apps) and the
 * app workspace (one app). Reconciles both secrets and config maps in a single
 * call. Owns the confirm dialog and the request; the result is presented in a
 * report modal (see `KubeSyncReportModal`) that stays open until the user
 * closes it. Scope is enforced by the backend: the sync only touches apps the
 * caller can access.
 */
export function useKubeSync() {
  const dialog = useDialog()
  const notification = useNotification()

  const syncing = ref(false)

  function confirmSync(options: SyncOptions = {}): void {
    const scope = options.appName
      ? `application "${options.appName}"`
      : 'all accessible applications'
    dialog.warning({
      title: 'Sync to Kubernetes',
      content:
        `Active secrets, config maps and their items of ${scope} will be ` +
        'applied to the cluster: missing resources are created, changed ones ' +
        'updated, and kusec-managed resources without active records are ' +
        'deleted. Continue?',
      positiveText: 'Sync',
      negativeText: 'Cancel',
      onPositiveClick: () => {
        void runSync(options.appId, scope)
      },
    })
  }

  async function runSync(appId: string | undefined, scope: string): Promise<void> {
    syncing.value = true
    try {
      const rep = await syncKube(appId)
      report.value = {
        scope,
        finishedAt: new Date(),
        secrets: rep.secrets,
        configmaps: rep.configmaps,
      }
      reportVisible.value = true
    } catch (error) {
      const detail = syncErrorText(error)
      // Persistent (duration: 0) so a one-shot toast can't be missed — and the
      // sync is idempotent, so re-running is always safe.
      notification.error({
        title: 'Kubernetes sync failed',
        content: () =>
          h('div', { style: 'display:flex;flex-direction:column;gap:8px' }, [
            h('div', { style: 'word-break:break-word' }, detail),
            h(
              'div',
              { style: 'opacity:.7' },
              'The deploy may still be completing on the server. ' +
                'Re-running the sync is safe.',
            ),
          ]),
        duration: 0,
      })
    } finally {
      syncing.value = false
    }
  }

  return { syncing, confirmSync, report, reportVisible }
}
