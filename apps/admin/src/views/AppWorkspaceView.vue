<script setup lang="ts">
import { computed, h, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NDataTable,
  NEllipsis,
  NEmpty,
  NFlex,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { ApiError } from '@/api/http'
import { deleteApp, getApp } from '@/api/app'
import { deleteSecret, listSecrets } from '@/api/secret'
import type { AppMain, SecretMain } from '@/api/types'
import { useAppsStore } from '@/stores/apps'

import AppFormModal from '@/components/app/AppFormModal.vue'
import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'
import SecretFormModal from '@/components/secret/SecretFormModal.vue'
import SecretItemsPanel from '@/components/secret/SecretItemsPanel.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()

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

// Auto-expand every secret on load when there are at most this many.
const AUTO_EXPAND_MAX = 5

const rows = ref<SecretMain[]>([])
const loading = ref(false)
const expandedKeys = ref<string[]>([])

// Broadcast a one-shot reveal/hide command; each items panel applies it to its
// own per-row flags, so individual rows stay togglable afterwards.
const showAll = ref(false)
const revealCommand = ref<{ action: 'show' | 'hide'; seq: number }>({
  action: 'hide',
  seq: 0,
})
provide('itemsRevealCommand', revealCommand)

const allExpanded = computed(
  () => rows.value.length > 0 && expandedKeys.value.length === rows.value.length,
)

function broadcastReveal(show: boolean): void {
  showAll.value = show
  revealCommand.value = {
    action: show ? 'show' : 'hide',
    seq: revealCommand.value.seq + 1,
  }
}

function toggleExpandAll(): void {
  if (allExpanded.value) {
    // Collapsing all secrets also hides all revealed values.
    expandedKeys.value = []
    broadcastReveal(false)
  } else {
    expandedKeys.value = rows.value.map((r) => r.id)
  }
}

function toggleRevealAll(): void {
  broadcastReveal(!showAll.value)
}

const editing = ref<SecretMain | null>(null)
const showForm = ref(false)

const detailId = ref<string | null>(null)
const showDetail = ref(false)

const showAppForm = ref(false)

function openAppEdit(): void {
  if (app.value) showAppForm.value = true
}

async function onAppSaved(): Promise<void> {
  await appsStore.refresh()
  await ensureApp()
}

function confirmAppDelete(): void {
  const current = app.value
  if (!current) return
  dialog.warning({
    title: 'Delete application',
    content: `Delete "${current.name}"? This also deletes all of its secrets and items.`,
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        await deleteApp(current.id)
        message.success('Application deleted')
        await appsStore.refresh()
        void router.push({ name: 'app-home' })
      } catch (error) {
        message.error(
          error instanceof ApiError ? error.message : 'Failed to delete application',
        )
      }
    },
  })
}

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
    const rep = await listSecrets({ app_id: appId.value })
    rows.value = rep.results ?? []
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to load secrets')
  } finally {
    loading.value = false
  }
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
          style: 'justify-content: flex-start; padding: 10px 0;',
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
    await ensureApp()
    await fetchSecrets()
    // Few secrets → expand them all by default for quicker overview.
    if (rows.value.length && rows.value.length <= AUTO_EXPAND_MAX) {
      expandedKeys.value = rows.value.map((r) => r.id)
    }
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
          <NTag v-if="app?.slug_name" size="small">
            {{ app.slug_name }}
          </NTag>
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
        <NSpace v-if="app" :size="8">
          <NButton size="small" @click="openAppEdit">Edit</NButton>
          <NButton size="small" type="error" secondary @click="confirmAppDelete">
            Delete
          </NButton>
        </NSpace>
      </NFlex>
      <NText v-if="app?.description" depth="3" style="display: block; margin-top: 6px">
        {{ app.description }}
      </NText>
    </NCard>

    <NCard title="Secrets">
      <template #header-extra>
        <NSpace :size="8" align="center">
          <NButton
            v-if="rows.length"
            size="small"
            tertiary
            @click="toggleExpandAll"
          >
            {{ allExpanded ? 'Collapse all' : 'Expand all' }}
          </NButton>
          <NButton
            v-if="rows.length"
            size="small"
            tertiary
            :type="showAll ? 'warning' : 'default'"
            @click="toggleRevealAll"
          >
            {{ showAll ? 'Hide all' : 'Show all' }}
          </NButton>
          <NButton type="primary" @click="openCreate">New secret</NButton>
        </NSpace>
      </template>

      <NDataTable
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: SecretMain) => row.id"
        :pagination="false"
        :theme-overrides="{ tdColorHover: 'transparent' }"
        :expanded-row-keys="expandedKeys"
        @update:expanded-row-keys="(keys: (string | number)[]) => (expandedKeys = keys.map(String))"
      />
    </NCard>

    <AppFormModal v-model:show="showAppForm" :app="app" @saved="onAppSaved" />
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
