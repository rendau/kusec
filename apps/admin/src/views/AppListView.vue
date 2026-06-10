<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
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
import { deleteApp, listApps } from '@/api/app'
import type { AppListReq, AppMain } from '@/api/types'

import AppDetailDrawer from '@/components/app/AppDetailDrawer.vue'
import AppFormModal from '@/components/app/AppFormModal.vue'

const message = useMessage()
const router = useRouter()

const rows = ref<AppMain[]>([])
const loading = ref(false)

const filters = reactive<{ search: string; active: boolean | null }>({
  search: '',
  active: null,
})

const activeOptions: SelectOption[] = [
  { label: 'All statuses', value: 'all' },
  { label: 'Active', value: 'active' },
  { label: 'Inactive', value: 'inactive' },
]

const pagination = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
})

// ── Detail drawer + create/edit modal state ──────────────
const detailId = ref<string | null>(null)
const showDetail = ref(false)

const editing = ref<AppMain | null>(null)
const showForm = ref(false)

async function fetchApps(): Promise<void> {
  loading.value = true
  try {
    const req: AppListReq = {
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

    const rep = await listApps(req)
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
  } catch (error) {
    const reason =
      error instanceof ApiError ? error.message : 'Failed to load applications'
    message.error(reason)
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  pagination.page = 1
  void fetchApps()
}

function onActiveChange(value: string): void {
  filters.active = value === 'all' ? null : value === 'active'
  applyFilters()
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchApps()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchApps()
}

function openDetail(row: AppMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: AppMain): void {
  editing.value = row
  showForm.value = true
}

function openSecrets(row: AppMain): void {
  void router.push({ name: 'secret-list', query: { app_id: row.id } })
}

async function removeApp(row: AppMain): Promise<void> {
  try {
    await deleteApp(row.id)
    message.success('Application deleted')
    // Step back a page if we just removed the last row on it.
    if (rows.value.length === 1 && pagination.page > 1) {
      pagination.page -= 1
    }
    await fetchApps()
  } catch (error) {
    const reason =
      error instanceof ApiError ? error.message : 'Failed to delete application'
    message.error(reason)
  }
}

const columns: DataTableColumns<AppMain> = [
  {
    title: 'Name',
    key: 'name',
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', onClick: () => openDetail(row) },
        { default: () => row.name },
      ),
  },
  {
    title: 'Namespace',
    key: 'namespace',
    render: (row) => h(NTag, { size: 'small' }, { default: () => row.namespace }),
  },
  {
    title: 'Description',
    key: 'description',
    render: (row) =>
      row.description
        ? h(NEllipsis, { style: 'max-width: 320px' }, { default: () => row.description })
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
          { size: 'small', onClick: () => openSecrets(row) },
          { default: () => 'Secrets' },
        ),
        h(
          NButton,
          { size: 'small', onClick: () => openEdit(row) },
          { default: () => 'Edit' },
        ),
        h(
          NPopconfirm,
          { onPositiveClick: () => removeApp(row) },
          {
            trigger: () =>
              h(
                NButton,
                { size: 'small', type: 'error', secondary: true },
                { default: () => 'Delete' },
              ),
            default: () => `Delete "${row.name}"?`,
          },
        ),
      ]),
  },
]

onMounted(fetchApps)
</script>

<template>
  <NSpace vertical :size="16">
    <NCard title="Applications">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">New application</NButton>
      </template>

      <NFlex justify="space-between" align="center" wrap style="margin-bottom: 16px">
        <NInput
          v-model:value="filters.search"
          placeholder="Search by name or namespace"
          clearable
          style="max-width: 320px"
          @keyup.enter="applyFilters"
          @clear="applyFilters"
        />
        <NSelect
          :default-value="'all'"
          :options="activeOptions"
          style="width: 180px"
          @update:value="onActiveChange"
        />
      </NFlex>

      <NDataTable
        remote
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: AppMain) => row.id"
        :pagination="pagination"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NCard>

    <AppDetailDrawer v-model:show="showDetail" :app-id="detailId" />
    <AppFormModal
      v-model:show="showForm"
      :app="editing"
      @saved="fetchApps"
    />
  </NSpace>
</template>
