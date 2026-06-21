<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NBreadcrumb,
  NBreadcrumbItem,
  NButton,
  NCard,
  NEmpty,
  NIcon,
  NPageHeader,
  NSpace,
  NTabPane,
  NTabs,
  NTag,
  NText,
} from 'naive-ui'
import { storeToRefs } from 'pinia'
import { CloudUpload, Pencil, Plus, Trash } from '@vicons/tabler'

import { getApp } from '@/api/app'
import type { AppMain } from '@/api/types'
import { useKubeSync } from '@/composables/useKubeSync'
import { useAppsStore } from '@/stores/apps'
import { useAuthStore } from '@/stores/auth'

import AppDeleteModal from '@/components/app/AppDeleteModal.vue'
import AppFormModal from '@/components/app/AppFormModal.vue'
import ConfigMapsSection from '@/components/configmap/ConfigMapsSection.vue'
import SecretsSection from '@/components/secret/SecretsSection.vue'

const route = useRoute()
const router = useRouter()

const appsStore = useAppsStore()

// Creating/editing/deleting applications is admin-only on the backend; regular
// users still browse apps and manage secrets/items within their access scope.
const { isAdmin } = storeToRefs(useAuthStore())

// Syncing this app's secrets to the cluster is allowed for anyone with access.
const { syncing, confirmSync } = useKubeSync()

const appId = computed(() =>
  typeof route.params.id === 'string' ? route.params.id : null,
)

// Prefer the cached app (reactive to renames); fall back to a direct fetch
// for deep links before the store has loaded.
const fetchedApp = ref<AppMain | null>(null)
const app = computed<AppMain | null>(
  () => (appId.value ? appsStore.getById(appId.value) : null) ?? fetchedApp.value,
)

// Active tab: switch between the app's secrets and config maps.
const activeTab = ref<'secrets' | 'configmaps'>('configmaps')

// Live counts are read from each section so the tab labels and delete modal
// stay in sync as secrets/config maps are created or removed.
const secretsRef = ref<InstanceType<typeof SecretsSection> | null>(null)
const configMapsRef = ref<InstanceType<typeof ConfigMapsSection> | null>(null)
const secretCount = computed(() => secretsRef.value?.count ?? 0)
const configMapCount = computed(() => configMapsRef.value?.count ?? 0)

const showAppForm = ref(false)
const showAppDelete = ref(false)

function openAppEdit(): void {
  if (app.value) showAppForm.value = true
}

async function onAppSaved(createdId?: string): Promise<void> {
  await appsStore.refresh()
  // Созданное приложение (из пустого состояния) сразу открываем.
  if (createdId) {
    void router.push({ name: 'app-workspace', params: { id: createdId } })
    return
  }
  await ensureApp()
}

async function onAppDeleted(): Promise<void> {
  await appsStore.refresh()
  void router.push({ name: 'app-home' })
}

async function ensureApp(): Promise<void> {
  fetchedApp.value = null
  if (!appId.value) return
  await appsStore.ensureLoaded()
  if (!appsStore.getById(appId.value)) {
    try {
      fetchedApp.value = await getApp(appId.value)
    } catch {
      fetchedApp.value = null
    }
  }
}

// React to route param changes (switching apps in the sidebar). Each section
// reloads its own list off the `app-id` prop; here we only resolve the app.
watch(
  appId,
  () => {
    void ensureApp()
  },
  { immediate: true },
)
</script>

<template>
  <div v-if="!appId" class="workspace-empty">
    <NEmpty
      :description="
        isAdmin
          ? 'Select an application from the sidebar, or create one.'
          : 'Select an application from the sidebar.'
      "
    >
      <template #extra>
        <NSpace vertical :size="12" align="center">
          <NText depth="3">Applications hold secrets, and secrets hold items.</NText>
          <NButton v-if="isAdmin" type="primary" @click="showAppForm = true">
            <template #icon>
              <NIcon :component="Plus" />
            </template>
            Create application
          </NButton>
        </NSpace>
      </template>
    </NEmpty>
    <AppFormModal v-model:show="showAppForm" :app="null" @saved="onAppSaved" />
  </div>

  <NSpace v-else vertical :size="16">
    <NCard class="stack-header">
      <NPageHeader>
        <template #header>
          <NBreadcrumb>
            <NBreadcrumbItem @click="router.push({ name: 'app-home' })">
              Applications
            </NBreadcrumbItem>
            <NBreadcrumbItem v-if="app?.namespace">
              {{ app.namespace }}
            </NBreadcrumbItem>
            <NBreadcrumbItem>{{ app?.name ?? '…' }}</NBreadcrumbItem>
          </NBreadcrumb>
        </template>

        <template #title>{{ app?.name ?? '…' }}</template>

        <template #extra>
          <NSpace v-if="app" :size="8">
            <NButton
              size="small"
              :loading="syncing"
              @click="confirmSync({ appId: app.id, appName: app.name })"
            >
              <template #icon>
                <NIcon :component="CloudUpload" />
              </template>
              Sync to Kubernetes
            </NButton>
            <template v-if="isAdmin">
              <NButton size="small" @click="openAppEdit">
                <template #icon>
                  <NIcon :component="Pencil" />
                </template>
                Edit
              </NButton>
              <NButton
                size="small"
                type="error"
                secondary
                @click="showAppDelete = true"
              >
                <template #icon>
                  <NIcon :component="Trash" />
                </template>
                Delete
              </NButton>
            </template>
          </NSpace>
        </template>

        <NSpace align="center" :size="8" style="margin-top: 4px">
          <NTag v-if="app?.slug_name" size="small" class="mono-tag">
            {{ app.slug_name }}
          </NTag>
          <NTag v-if="app?.namespace" size="small" type="info">
            {{ app.namespace }}
          </NTag>
          <NTag v-if="app" size="small" :type="app.active ? 'success' : 'default'">
            {{ app.active ? 'Active' : 'Inactive' }}
          </NTag>
          <NText v-if="secretCount" depth="3" style="font-size: 12px">
            {{ secretCount }} secret{{ secretCount === 1 ? '' : 's' }}
          </NText>
        </NSpace>
        <NText
          v-if="app?.description"
          depth="3"
          style="display: block; margin-top: 6px"
        >
          {{ app.description }}
        </NText>
      </NPageHeader>
    </NCard>

    <NCard class="stack-header">
      <NTabs
        v-model:value="activeTab"
        type="line"
        animated
        display-directive="show:lazy"
        pane-class="workspace-tab-pane"
      >
        <NTabPane name="configmaps">
          <template #tab>
            Config maps
            <NTag
              v-if="configMapCount"
              size="small"
              round
              :bordered="false"
              class="workspace-tab__count"
            >
              {{ configMapCount }}
            </NTag>
          </template>
          <ConfigMapsSection v-if="appId" ref="configMapsRef" :app-id="appId" />
        </NTabPane>

        <NTabPane name="secrets">
          <template #tab>
            Secrets
            <NTag
              v-if="secretCount"
              size="small"
              round
              :bordered="false"
              class="workspace-tab__count"
            >
              {{ secretCount }}
            </NTag>
          </template>
          <SecretsSection v-if="appId" ref="secretsRef" :app-id="appId" />
        </NTabPane>
      </NTabs>
    </NCard>

    <AppFormModal v-model:show="showAppForm" :app="app" @saved="onAppSaved" />
    <AppDeleteModal
      v-model:show="showAppDelete"
      :app="app"
      :secret-count="secretCount"
      @deleted="onAppDeleted"
    />
  </NSpace>
</template>

<style scoped>
.workspace-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}

.mono-tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

/* Count badge next to a tab label. */
.workspace-tab__count {
  margin-left: 6px;
}
</style>
