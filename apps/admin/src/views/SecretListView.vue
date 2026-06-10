<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { LocationQuery } from 'vue-router'
import {
  NButton,
  NCard,
  NDataTable,
  NEllipsis,
  NFlex,
  NInput,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'

import { ApiError } from '@/api/http'
import { deleteSecret, listSecrets } from '@/api/secret'
import type { SecretListReq, SecretMain } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'

import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'
import SecretFormModal from '@/components/secret/SecretFormModal.vue'

const message = useMessage()
const route = useRoute()
const router = useRouter()

const {
  options: appOptions,
  loading: appsLoading,
  search: searchApps,
  ensure: ensureApp,
  nameOf: appNameOf,
} = useAppOptions()

const rows = ref<SecretMain[]>([])
const loading = ref(false)

const filters = reactive<{
  search: string
  active: boolean | null
  appId: string | null
}>({
  search: '',
  active: null,
  // Deep-link: /secret?app_id=<id> pre-filters by application.
  appId: typeof route.query.app_id === 'string' ? route.query.app_id : null,
})

const activeOptions: SelectOption[] = [
  { label: 'All statuses', value: 'all' },
  { label: 'Active', value: 'active' },
  { label: 'Inactive', value: 'inactive' },
]

const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
})

const detailId = ref<string | null>(null)
const showDetail = ref(false)

const editing = ref<SecretMain | null>(null)
const showForm = ref(false)

async function fetchSecrets(): Promise<void> {
  loading.value = true
  try {
    const req: SecretListReq = {
      list_params: {
        // API pagination is zero-based; naive-ui's table is 1-based.
        page: pagination.page - 1,
        page_size: pagination.pageSize,
        with_total_count: true,
      },
    }
    const search = filters.search.trim()
    if (search) req.search = search
    if (filters.active !== null) req.active = filters.active
    if (filters.appId) req.app_id = filters.appId

    const rep = await listSecrets(req)
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)

    // Resolve app names for the rows on screen.
    const ids = [...new Set(rows.value.map((r) => r.app_id))]
    await Promise.all(ids.map((id) => ensureApp(id)))
  } catch (error) {
    message.error(
      error instanceof ApiError ? error.message : 'Failed to load secrets',
    )
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  pagination.page = 1
  void fetchSecrets()
}

function onActiveChange(value: string): void {
  filters.active = value === 'all' ? null : value === 'active'
  applyFilters()
}

function onAppFilterChange(value: string | null): void {
  filters.appId = value ?? null
  // Keep the URL in sync so the filtered view is shareable.
  void router.replace({
    query: value ? { ...route.query, app_id: value } : omitAppId(route.query),
  })
  applyFilters()
}

function omitAppId(query: LocationQuery): LocationQuery {
  const { app_id: _omit, ...rest } = query
  return rest
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchSecrets()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchSecrets()
}

function openDetail(row: SecretMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: SecretMain): void {
  editing.value = row
  showForm.value = true
}

function openItems(row: SecretMain): void {
  void router.push({ name: 'item-list', query: { secret_id: row.id } })
}

async function removeSecret(row: SecretMain): Promise<void> {
  try {
    await deleteSecret(row.id)
    message.success('Secret deleted')
    if (rows.value.length === 1 && pagination.page > 1) {
      pagination.page -= 1
    }
    await fetchSecrets()
  } catch (error) {
    message.error(
      error instanceof ApiError ? error.message : 'Failed to delete secret',
    )
  }
}

const columns: DataTableColumns<SecretMain> = [
  {
    title: 'Slug',
    key: 'slug_name',
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', onClick: () => openDetail(row) },
        { default: () => row.slug_name },
      ),
  },
  {
    title: 'Application',
    key: 'app_id',
    render: (row) =>
      h(NTag, { size: 'small', type: 'info' }, { default: () => appNameOf(row.app_id) }),
  },
  {
    title: 'Description',
    key: 'description',
    render: (row) =>
      row.description
        ? h(NEllipsis, { style: 'max-width: 280px' }, { default: () => row.description })
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
    width: 300,
    render: (row) =>
      h(NSpace, { size: 8 }, () => [
        h(
          NButton,
          { size: 'small', onClick: () => openDetail(row) },
          { default: () => 'View' },
        ),
        h(
          NButton,
          { size: 'small', onClick: () => openItems(row) },
          { default: () => 'Items' },
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
            default: () => `Delete "${row.slug_name}"?`,
          },
        ),
      ]),
  },
]

onMounted(async () => {
  await searchApps()
  if (filters.appId) await ensureApp(filters.appId)
  await fetchSecrets()
})
</script>

<template>
  <NSpace vertical :size="16">
    <NCard title="Secrets">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">New secret</NButton>
      </template>

      <NFlex align="center" wrap :size="12" style="margin-bottom: 16px">
        <NInput
          v-model:value="filters.search"
          placeholder="Search by slug"
          clearable
          style="max-width: 280px"
          @keyup.enter="applyFilters"
          @clear="applyFilters"
        />
        <NSelect
          :value="filters.appId"
          :options="appOptions"
          :loading="appsLoading"
          filterable
          remote
          clearable
          placeholder="Filter by application"
          style="width: 240px"
          @search="searchApps"
          @update:value="onAppFilterChange"
        />
        <NSelect
          :default-value="'all'"
          :options="activeOptions"
          style="width: 160px"
          @update:value="onActiveChange"
        />
      </NFlex>

      <NDataTable
        remote
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: SecretMain) => row.id"
        :pagination="pagination"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NCard>

    <SecretDetailDrawer v-model:show="showDetail" :secret-id="detailId" />
    <SecretFormModal
      v-model:show="showForm"
      :secret="editing"
      :default-app-id="filters.appId"
      @saved="fetchSecrets"
    />
  </NSpace>
</template>
