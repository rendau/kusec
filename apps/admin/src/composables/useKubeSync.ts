import { h, ref } from 'vue'
import { useDialog, useNotification } from 'naive-ui'

import { ApiError, apiErrorMessage } from '@/api/http'
import { syncKube } from '@/api/kube'
import type { KubeSyncConfigMapsRep, KubeSyncSecretsRep } from '@/api/types'

/** Human-readable hints for the sentinel error codes the sync may return. */
const SYNC_ERROR_HINTS: Record<string, string> = {
  not_in_cluster: 'The service is not running inside a Kubernetes cluster',
  sync_in_progress: 'A sync is already in progress, try again in a moment',
  no_permission: 'You do not have access to the requested application',
  // Returned by the API layer when no response arrives in time (proxy timeout).
  service_not_available:
    'No response received in time — the deploy is likely still running on the server',
}

/** Coerce an int64-as-string counter into a number for display. */
function toCount(value: number | string | undefined): number {
  const parsed = Number(value ?? 0)
  return Number.isFinite(parsed) ? parsed : 0
}

interface SyncOptions {
  /** Sync a single application; omit to sync every accessible application. */
  appId?: string
  /** App name for the confirmation copy (only relevant with `appId`). */
  appName?: string
}

/** One-line per-kind summary ("created X · updated Y · …"). */
function summarize(
  rep: KubeSyncSecretsRep | KubeSyncConfigMapsRep,
): { line: string; errors: string[] } {
  return {
    line: [
      `created ${rep.created?.length ?? 0}`,
      `updated ${rep.updated?.length ?? 0}`,
      `deleted ${rep.deleted?.length ?? 0}`,
      `unchanged ${toCount(rep.unchanged)}`,
    ].join(' · '),
    errors: rep.errors ?? [],
  }
}

/**
 * Sync report VNode: errors first as a highlighted list (so real failures are
 * easy to spot), then the compact per-kind summary. Used for the
 * finished-with-errors notification.
 */
function renderReport(errors: string[], summaryLines: string[]) {
  return h('div', { style: 'display:flex;flex-direction:column;gap:8px' }, [
    h(
      'ul',
      { style: 'margin:0;padding-left:18px;color:var(--n-text-color)' },
      errors.map((e) =>
        h('li', { style: 'margin-bottom:2px;word-break:break-word' }, e),
      ),
    ),
    h(
      'div',
      { style: 'opacity:.7;white-space:pre-line' },
      summaryLines.join('\n'),
    ),
  ])
}

/**
 * Shared "sync to Kubernetes" flow used by the dashboard (all apps) and the
 * app workspace (one app). Reconciles both secrets and config maps in a single
 * call. Owns the confirm dialog, the request, and the result/error
 * notifications. Scope is enforced by the backend: the sync only touches apps
 * the caller can access.
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
        void runSync(options.appId)
      },
    })
  }

  async function runSync(appId?: string): Promise<void> {
    syncing.value = true
    try {
      const rep = await syncKube(appId)
      const secrets = summarize(rep.secrets)
      const configmaps = summarize(rep.configmaps)
      const summaryLines = [
        `Secrets: ${secrets.line}`,
        `Config maps: ${configmaps.line}`,
      ]
      const errors = [...secrets.errors, ...configmaps.errors]

      if (errors.length) {
        notification.warning({
          title: `Kubernetes sync finished with ${errors.length} error${errors.length > 1 ? 's' : ''}`,
          // Errors need attention — dismissed manually.
          content: () => renderReport(errors, summaryLines),
          duration: 0,
        })
      } else {
        notification.success({
          title: 'Kubernetes sync finished',
          content: summaryLines.join('\n'),
          duration: 6000,
        })
      }
    } catch (error) {
      const hint =
        error instanceof ApiError ? SYNC_ERROR_HINTS[error.code] : undefined
      const detail = hint ?? apiErrorMessage(error, 'Kubernetes sync failed')
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

  return { syncing, confirmSync }
}
