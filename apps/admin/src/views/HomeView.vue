<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NEmpty,
  NGrid,
  NGridItem,
  NIcon,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  NText,
  NThing,
  useDialog,
  useMessage,
  useNotification,
} from 'naive-ui'
import { Apps, CloudUpload, Key, Lock, Plus, Users } from '@vicons/tabler'
import type { Component } from 'vue'

import { ApiError, apiErrorMessage } from '@/api/http'
import { getDashboard } from '@/api/dashboard'
import { syncKubeSecrets } from '@/api/kube'
import type { DashboardRecentSecret, DashboardRep } from '@/api/types'
import { useAppsStore } from '@/stores/apps'
import { useAuthStore } from '@/stores/auth'
import { formatDate } from '@/utils/format'

import AppFormModal from '@/components/app/AppFormModal.vue'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const notification = useNotification()
const appsStore = useAppsStore()
const authStore = useAuthStore()

const dashboard = ref<DashboardRep | null>(null)
const loading = ref(false)

const showAppForm = ref(false)

interface StatCard {
  key: string
  label: string
  icon: Component
  total: number | null
  active: number | null
}

function toCount(value: number | string | undefined): number | null {
  if (value == null) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

const stats = computed<StatCard[]>(() => {
  const rep = dashboard.value
  const card = (
    key: string,
    label: string,
    icon: Component,
    count?: { total: number | string; active: number | string },
  ): StatCard => ({
    key,
    label,
    icon,
    total: toCount(count?.total),
    active: toCount(count?.active),
  })
  return [
    card('apps', 'Applications', Apps, rep?.app),
    card('secrets', 'Secrets', Lock, rep?.secret),
    card('items', 'Items', Key, rep?.item),
    card('users', 'Users', Users, rep?.usr),
  ]
})

const recentSecrets = computed<DashboardRecentSecret[]>(
  () => dashboard.value?.recent_secrets ?? [],
)

async function loadDashboard(): Promise<void> {
  loading.value = true
  try {
    dashboard.value = await getDashboard()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load dashboard'))
  } finally {
    loading.value = false
  }
}

function openSecret(secret: DashboardRecentSecret): void {
  void router.push({ name: 'app-workspace', params: { id: secret.app_id } })
}

async function onAppSaved(): Promise<void> {
  await appsStore.refresh()
  await loadDashboard()
}

// ── Sync to Kubernetes ─────────────────────────────────

const syncing = ref(false)

const SYNC_ERROR_HINTS: Record<string, string> = {
  not_in_cluster: 'The service is not running inside a Kubernetes cluster',
  sync_in_progress: 'A sync is already in progress, try again in a moment',
  no_permission: 'Only administrators can sync secrets',
}

function confirmKubeSync(): void {
  dialog.warning({
    title: 'Sync secrets to Kubernetes',
    content:
      'Active applications, secrets and items will be applied to the cluster: ' +
      'missing secrets are created, changed ones updated, and kusec-managed ' +
      'secrets without active records are deleted. Continue?',
    positiveText: 'Sync',
    negativeText: 'Cancel',
    onPositiveClick: () => {
      void runKubeSync()
    },
  })
}

async function runKubeSync(): Promise<void> {
  syncing.value = true
  try {
    const rep = await syncKubeSecrets()
    const summary = [
      `created ${rep.created?.length ?? 0}`,
      `updated ${rep.updated?.length ?? 0}`,
      `deleted ${rep.deleted?.length ?? 0}`,
      `unchanged ${toCount(rep.unchanged) ?? 0}`,
    ].join(' · ')

    if (rep.errors?.length) {
      notification.warning({
        title: 'Kubernetes sync finished with errors',
        content: `${summary}\n\n${rep.errors.join('\n')}`,
        // Ошибки требуют внимания — закрывается вручную.
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

onMounted(() => {
  void loadDashboard()
})
</script>

<template>
  <NSpace vertical :size="16">
    <NGrid cols="2 m:4" responsive="screen" :x-gap="16" :y-gap="16">
      <NGridItem v-for="stat in stats" :key="stat.key">
        <NCard>
          <NStatistic :label="stat.label">
            <template #prefix>
              <NIcon :component="stat.icon" :depth="3" />
            </template>
            {{ stat.total ?? '—' }}
            <template #suffix>
              <NText
                v-if="stat.total !== null && stat.active !== null && stat.active < stat.total"
                depth="3"
                style="font-size: 13px"
              >
                / {{ stat.active }} active
              </NText>
            </template>
          </NStatistic>
        </NCard>
      </NGridItem>
    </NGrid>

    <NCard title="Recently updated secrets">
      <template #header-extra>
        <NSpace :size="8">
          <NButton
            v-if="authStore.isAdmin"
            size="small"
            :loading="syncing"
            @click="confirmKubeSync"
          >
            <template #icon>
              <NIcon :component="CloudUpload" />
            </template>
            Sync to Kubernetes
          </NButton>
          <NButton size="small" type="primary" @click="showAppForm = true">
            <template #icon>
              <NIcon :component="Plus" />
            </template>
            New application
          </NButton>
        </NSpace>
      </template>

      <NSpin :show="loading">
        <NEmpty
          v-if="!loading && recentSecrets.length === 0"
          description="No secrets yet — create an application and add one."
        />
        <NSpace v-else vertical :size="0">
          <button
            v-for="secret in recentSecrets"
            :key="secret.id"
            type="button"
            class="recent-row"
            @click="openSecret(secret)"
          >
            <NThing>
              <template #header>
                <span class="recent-row__slug">{{ secret.slug_name }}</span>
              </template>
              <template #header-extra>
                <NText depth="3" style="font-size: 12px">
                  {{ formatDate(secret.updated_at) }}
                </NText>
              </template>
              <template #description>
                <NSpace :size="6" align="center">
                  <NTag size="tiny" type="info" :bordered="false">
                    {{ secret.app_name || secret.app_id }}
                  </NTag>
                  <NTag size="tiny" :bordered="false">
                    {{ toCount(secret.item_count) ?? 0 }}
                    item{{ toCount(secret.item_count) === 1 ? '' : 's' }}
                  </NTag>
                  <NTag
                    v-if="!secret.active"
                    size="tiny"
                    :bordered="false"
                    type="warning"
                  >
                    inactive
                  </NTag>
                  <NText v-if="secret.description" depth="3" style="font-size: 12px">
                    {{ secret.description }}
                  </NText>
                </NSpace>
              </template>
            </NThing>
          </button>
        </NSpace>
      </NSpin>
    </NCard>

    <AppFormModal v-model:show="showAppForm" :app="null" @saved="onAppSaved" />
  </NSpace>
</template>

<style scoped>
.recent-row {
  display: block;
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.recent-row:hover {
  background: rgba(128, 128, 128, 0.08);
}

.recent-row + .recent-row {
  border-top: 1px solid rgba(128, 128, 128, 0.14);
  border-radius: 0;
}

.recent-row__slug {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13.5px;
  font-weight: 600;
}
</style>
