<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  NButton,
  NCard,
  NDataTable,
  NEllipsis,
  NEmpty,
  NFlex,
  NInput,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { ApiError } from '@/api/http'
import { getApp } from '@/api/app'
import { deleteSecret, listSecrets } from '@/api/secret'
import type { AppMain, SecretListReq, SecretMain } from '@/api/types'
import { useAppsStore } from '@/stores/apps'

import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'
import SecretFormModal from '@/components/secret/SecretFormModal.vue'
import SecretItemsPanel from '@/components/secret/SecretItemsPanel.vue'

const route = useRoute()
const message = useMessage()

const appsStore = useAppsStore()
const { apps } = storeToRefs(appsStore)

const appId = computed(() =>
  typeof route.params.id === 'string' ? route.params.id : null,
)

// Prefer the cached app (reactive to renames); fall back to a direct fetch
// for deep links before the store has loaded.
const fetchedApp = ref<AppMain | null>(null)
const app = computed<AppMain | null>(
  () => (appId.value ? appsStore.getById(appId.value) : null) ?? fetchedApp.value,
)

const rows = ref<SecretMain[]>([])
const loading = ref(false)
const search = ref('')
const expandedKeys = ref<string[]>([])

const editing = ref<SecretMain | null>(null)
const showForm = ref(false)

const detailId = ref<string | null>(null)
const showDetail = ref(false)

function openDetail(row: SecretMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function toggleExpand(row: SecretMain): void {
  const set = new Set(expandedKeys.value)
  if (set.has(row.id)) set.delete(row.id)
  else set.add(row.id)
  expandedKeys.value = [...set]
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

async function fetchSecrets(): Promise<void> {
  if (!appId.value) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    // Scoped by app_id → backend returns all secrets, no pagination needed.
    const req: SecretListReq = { app_id: appId.value }
    const q = search.value.trim()
    if (q) req.search = q

    const rep = await listSecrets(req)
    rows.value = rep.results ?? []
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to load secrets')
  } finally {
    loading.value = false
  }
}

function applySearch(): void {
  void fetchSecrets()
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: SecretMain): void {
  editing.value = row
  showForm.value = true
}

async function removeSecret(row: SecretMain): Promise<void> {
  try {
    await deleteSecret(row.id)
    message.success('Secret deleted')
    await fetchSecrets()
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to delete secret')
  }
}

const columns = computed<DataTableColumns<SecretMain>>(() => [
  {
    type: 'expand',
    renderExpand: (row) => h(SecretItemsPanel, { secretId: row.id }),
  },
  {
    title: 'Slug',
    key: 'slug_name',
    render: (row) =>
      h(
        NButton,
        {
          text: true,
          type: 'primary',
          block: true,
          // Click the name to expand/collapse the row (View has its own button).
          style: 'justify-content: flex-start; padding: 10px 4px;',
          onClick: () => toggleExpand(row),
        },
        { default: () => row.slug_name },
      ),
  },
  {
    title: 'Description',
    key: 'description',
    render: (row) =>
      row.description
        ? h(
            NEllipsis,
            { style: 'max-width: 320px' },
            { default: () => row.description },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
  {
    title: 'Status',
    key: 'active',
    width: 110,
    render: (row) =>
      h(
        NTag,
        { type: row.active ? 'success' : 'default', size: 'small' },
        { default: () => (row.active ? 'Active' : 'Inactive') },
      ),
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 240,
    render: (row) =>
      h(NSpace, { size: 8 }, () => [
        h(
          NButton,
          { size: 'small', onClick: () => openDetail(row) },
          { default: () => 'View' },
        ),
        h(
          NButton,
          { size: 'small', onClick: () => openEdit(row) },
          { default: () => 'Edit' },
        ),
        h(
          NPopconfirm,
          { onPositiveClick: () => removeSecret(row) },
          {
            trigger: () =>
              h(
                NButton,
                { size: 'small', type: 'error', secondary: true },
                { default: () => 'Delete' },
              ),
            default: () => `Delete "${row.slug_name}"? This also deletes its items.`,
          },
        ),
      ]),
  },
])

// React to route param changes (switching apps in the sidebar).
watch(
  appId,
  async () => {
    expandedKeys.value = []
    search.value = ''
    await ensureApp()
    await fetchSecrets()
  },
  { immediate: true },
)

// If the store loads/changes after mount, keep the secrets in sync once.
watch(apps, () => {
  if (appId.value && !rows.value.length && !loading.value) void fetchSecrets()
})
</script>

<template>
  <div v-if="!appId" class="workspace-empty">
    <NEmpty description="Select an application from the sidebar, or create one.">
      <template #extra>
        <NText depth="3">Applications hold secrets, and secrets hold items.</NText>
      </template>
    </NEmpty>
  </div>

  <NSpace v-else vertical :size="16">
    <NCard>
      <NFlex align="center" justify="space-between" wrap>
        <NSpace align="center" :size="10">
          <NText strong style="font-size: 18px">{{ app?.name ?? '…' }}</NText>
          <NTag v-if="app?.namespace" size="small" type="info">
            {{ app.namespace }}
          </NTag>
          <NTag
            v-if="app"
            size="small"
            :type="app.active ? 'success' : 'default'"
          >
            {{ app.active ? 'Active' : 'Inactive' }}
          </NTag>
        </NSpace>
      </NFlex>
      <NText v-if="app?.description" depth="3" style="display: block; margin-top: 6px">
        {{ app.description }}
      </NText>
    </NCard>

    <NCard title="Secrets">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">New secret</NButton>
      </template>

      <NFlex style="margin-bottom: 16px">
        <NInput
          v-model:value="search"
          placeholder="Search by slug"
          clearable
          style="max-width: 280px"
          @keyup.enter="applySearch"
          @clear="applySearch"
        />
      </NFlex>

      <NDataTable
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: SecretMain) => row.id"
        :pagination="false"
        :expanded-row-keys="expandedKeys"
        @update:expanded-row-keys="(keys: (string | number)[]) => (expandedKeys = keys.map(String))"
      />
    </NCard>

    <SecretDetailDrawer v-model:show="showDetail" :secret-id="detailId" />
    <SecretFormModal
      v-model:show="showForm"
      :secret="editing"
      :default-app-id="appId"
      lock-app
      @saved="fetchSecrets"
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
</style>
