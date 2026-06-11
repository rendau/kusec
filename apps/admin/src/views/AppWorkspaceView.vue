<script setup lang="ts">
import { computed, h, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NBreadcrumb,
  NBreadcrumbItem,
  NButton,
  NCard,
  NDataTable,
  NEllipsis,
  NEmpty,
  NIcon,
  NPageHeader,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { storeToRefs } from 'pinia'
import {
  ChevronDown,
  ChevronUp,
  Eye,
  EyeOff,
  InfoCircle,
  Pencil,
  Plus,
  Trash,
} from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { getApp } from '@/api/app'
import { deleteSecret, listSecrets } from '@/api/secret'
import type { AppMain, SecretMain } from '@/api/types'
import { itemsRevealCommandKey } from '@/constants/injection'
import type { RevealCommand } from '@/constants/injection'
import { useAppsStore } from '@/stores/apps'

import AppDeleteModal from '@/components/app/AppDeleteModal.vue'
import AppFormModal from '@/components/app/AppFormModal.vue'
import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'
import SecretFormModal from '@/components/secret/SecretFormModal.vue'
import SecretItemsPanel from '@/components/secret/SecretItemsPanel.vue'

const route = useRoute()
const router = useRouter()
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

// Auto-expand every secret on load when there are at most this many.
const AUTO_EXPAND_MAX = 5

const rows = ref<SecretMain[]>([])
const loading = ref(false)
const expandedKeys = ref<string[]>([])

// Broadcast a one-shot reveal/hide command; each items panel applies it to its
// own per-row flags, so individual rows stay togglable afterwards.
const showAll = ref(false)
const revealCommand = ref<RevealCommand>({ action: 'hide', seq: 0 })
provide(itemsRevealCommandKey, revealCommand)

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
const showAppDelete = ref(false)

function openAppEdit(): void {
  if (app.value) showAppForm.value = true
}

async function onAppSaved(): Promise<void> {
  await appsStore.refresh()
  await ensureApp()
}

async function onAppDeleted(): Promise<void> {
  await appsStore.refresh()
  void router.push({ name: 'app-home' })
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
    message.error(apiErrorMessage(error, 'Failed to load secrets'))
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
    message.error(apiErrorMessage(error, 'Failed to delete secret'))
  }
}

function iconButton(
  icon: typeof Eye,
  tooltip: string,
  onClick: () => void,
  type?: 'error',
) {
  return h(
    NTooltip,
    {},
    {
      trigger: () =>
        h(
          NButton,
          {
            quaternary: true,
            circle: true,
            size: 'small',
            ...(type ? { type } : {}),
            'aria-label': tooltip,
            onClick,
          },
          { icon: () => h(NIcon, { component: icon }) },
        ),
      default: () => tooltip,
    },
  )
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
          style:
            'justify-content: flex-start; padding: 10px 0;' +
            'font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;',
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
    width: 140,
    render: (row) =>
      h(NSpace, { size: 4, wrapItem: false }, () => [
        iconButton(InfoCircle, 'Details', () => openDetail(row)),
        iconButton(Pencil, 'Edit', () => openEdit(row)),
        h(
          NPopconfirm,
          { onPositiveClick: () => removeSecret(row) },
          {
            trigger: () =>
              h(
                NButton,
                {
                  quaternary: true,
                  circle: true,
                  size: 'small',
                  type: 'error',
                  'aria-label': 'Delete secret',
                },
                { icon: () => h(NIcon, { component: Trash }) },
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
        <NSpace vertical :size="12" align="center">
          <NText depth="3">Applications hold secrets, and secrets hold items.</NText>
          <NButton type="primary" @click="showAppForm = true">
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
    <NCard>
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
            <NButton size="small" @click="openAppEdit">
              <template #icon>
                <NIcon :component="Pencil" />
              </template>
              Edit
            </NButton>
            <NButton size="small" type="error" secondary @click="showAppDelete = true">
              <template #icon>
                <NIcon :component="Trash" />
              </template>
              Delete
            </NButton>
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
          <NText v-if="rows.length" depth="3" style="font-size: 12px">
            {{ rows.length }} secret{{ rows.length === 1 ? '' : 's' }}
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

    <NCard title="Secrets">
      <template #header-extra>
        <NSpace :size="8" align="center">
          <NButton v-if="rows.length" size="small" tertiary @click="toggleExpandAll">
            <template #icon>
              <NIcon :component="allExpanded ? ChevronUp : ChevronDown" />
            </template>
            {{ allExpanded ? 'Collapse all' : 'Expand all' }}
          </NButton>
          <NButton
            v-if="rows.length"
            size="small"
            tertiary
            :type="showAll ? 'warning' : 'default'"
            @click="toggleRevealAll"
          >
            <template #icon>
              <NIcon :component="showAll ? EyeOff : Eye" />
            </template>
            {{ showAll ? 'Hide all' : 'Show all' }}
          </NButton>
          <NButton type="primary" @click="openCreate">
            <template #icon>
              <NIcon :component="Plus" />
            </template>
            New secret
          </NButton>
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
        @update:expanded-row-keys="
          (keys: (string | number)[]) => (expandedKeys = keys.map(String))
        "
      />
    </NCard>

    <AppFormModal v-model:show="showAppForm" :app="app" @saved="onAppSaved" />
    <AppDeleteModal
      v-model:show="showAppDelete"
      :app="app"
      :secret-count="rows.length"
      @deleted="onAppDeleted"
    />
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

.mono-tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
</style>
