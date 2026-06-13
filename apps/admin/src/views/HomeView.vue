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
  useMessage,
} from 'naive-ui'
import { Apps, Key, Lock, Plus, Users } from '@vicons/tabler'
import type { Component } from 'vue'

import { apiErrorMessage } from '@/api/http'
import { getDashboard } from '@/api/dashboard'
import type { DashboardRecentSecret, DashboardRep } from '@/api/types'
import { useAppsStore } from '@/stores/apps'
import { useAuthStore } from '@/stores/auth'
import { formatDate } from '@/utils/format'

import AppFormModal from '@/components/app/AppFormModal.vue'

const router = useRouter()
const message = useMessage()
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

async function onAppSaved(createdId?: string): Promise<void> {
  await appsStore.refresh()
  // Созданное приложение сразу открываем в workspace.
  if (createdId) {
    void router.push({ name: 'app-workspace', params: { id: createdId } })
    return
  }
  await loadDashboard()
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

    <NCard title="Recently updated secrets" class="stack-header">
      <template #header-extra>
        <NButton
          v-if="authStore.isAdmin"
          size="small"
          type="primary"
          @click="showAppForm = true"
        >
          <template #icon>
            <NIcon :component="Plus" />
          </template>
          New application
        </NButton>
      </template>

      <NSpin :show="loading">
        <NEmpty
          v-if="!loading && recentSecrets.length === 0"
          :description="
            authStore.isAdmin
              ? 'No secrets yet — create an application and add one.'
              : 'No secrets yet.'
          "
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
