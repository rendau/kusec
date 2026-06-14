import { ref } from 'vue'
import { useDialog, useMessage, useNotification } from 'naive-ui'

import { ApiError, apiErrorMessage } from '@/api/http'
import { syncKube } from '@/api/kube'
import type { KubeSyncConfigMapsRep, KubeSyncSecretsRep } from '@/api/types'

/** Human-readable hints for the sentinel error codes the sync may return. */
const SYNC_ERROR_HINTS: Record<string, string> = {
  not_in_cluster: 'The service is not running inside a Kubernetes cluster',
  sync_in_progress: 'A sync is already in progress, try again in a moment',
  no_permission: 'You do not have access to the requested application',
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
 * Shared "sync to Kubernetes" flow used by the dashboard (all apps) and the
 * app workspace (one app). Reconciles both secrets and config maps in a single
 * call. Owns the confirm dialog, the request, and the result/error
 * notifications. Scope is enforced by the backend: the sync only touches apps
 * the caller can access.
 */
export function useKubeSync() {
  const message = useMessage()
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
      const summary = `Secrets: ${secrets.line}\nConfig maps: ${configmaps.line}`
      const errors = [...secrets.errors, ...configmaps.errors]

      if (errors.length) {
        notification.warning({
          title: 'Kubernetes sync finished with errors',
          content: `${summary}\n\n${errors.join('\n')}`,
          // Errors need attention — dismissed manually.
          duration: 0,
        })
      } else {
        notification.success({
          title: 'Kubernetes sync finished',
          content: summary,
          duration: 6000,
        })
      }
    } catch (error) {
      const hint =
        error instanceof ApiError ? SYNC_ERROR_HINTS[error.code] : undefined
      message.error(hint ?? apiErrorMessage(error, 'Kubernetes sync failed'))
    } finally {
      syncing.value = false
    }
  }

  return { syncing, confirmSync }
}
