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
import { deleteItem, listItems } from '@/api/item'
import type { ItemListReq, ItemMain } from '@/api/types'
import { useSecretOptions } from '@/composables/useSecretOptions'

import ItemDetailDrawer from '@/components/item/ItemDetailDrawer.vue'
import ItemFormModal from '@/components/item/ItemFormModal.vue'

const message = useMessage()
const route = useRoute()
const router = useRouter()

const {
  options: secretOptions,
  loading: secretsLoading,
  search: searchSecrets,
  ensure: ensureSecret,
  nameOf: secretNameOf,
} = useSecretOptions()

const rows = ref<ItemMain[]>([])
const loading = ref(false)
const revealed = ref(new Set<string>())

const filters = reactive<{
  search: string
  active: boolean | null
  secretId: string | null
}>({
  search: '',
  active: null,
  // Deep-link: /item?secret_id=<id> pre-filters by secret.
  secretId: typeof route.query.secret_id === 'string' ? route.query.secret_id : null,
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

const detailId = ref<string | null>(null)
const showDetail = ref(false)

const editing = ref<ItemMain | null>(null)
const showForm = ref(false)

async function fetchItems(): Promise<void> {
  loading.value = true
  try {
    const req: ItemListReq = {
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
    if (filters.secretId) req.secret_id = filters.secretId

    const rep = await listItems(req)
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
    revealed.value = new Set()

    // Resolve secret names for the rows on screen.
    const ids = [...new Set(rows.value.map((r) => r.secret_id))]
    await Promise.all(ids.map((id) => ensureSecret(id)))
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to load items')
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  pagination.page = 1
  void fetchItems()
}

function onActiveChange(value: string): void {
  filters.active = value === 'all' ? null : value === 'active'
  applyFilters()
}

function onSecretFilterChange(value: string | null): void {
  filters.secretId = value ?? null
  void router.replace({
    query: value ? { ...route.query, secret_id: value } : omitSecretId(route.query),
  })
  applyFilters()
}

function omitSecretId(query: LocationQuery): LocationQuery {
  const { secret_id: _omit, ...rest } = query
  return rest
}

function toggleReveal(id: string): void {
  const next = new Set(revealed.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  revealed.value = next
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchItems()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchItems()
}

function openDetail(row: ItemMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: ItemMain): void {
  editing.value = row
  showForm.value = true
}

async function removeItem(row: ItemMain): Promise<void> {
  try {
    await deleteItem(row.id)
    message.success('Item deleted')
    if (rows.value.length === 1 && pagination.page > 1) {
      pagination.page -= 1
    }
    await fetchItems()
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to delete item')
  }
}

const columns: DataTableColumns<ItemMain> = [
  {
    title: 'Key',
    key: 'key',
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', onClick: () => openDetail(row) },
        { default: () => row.key },
      ),
  },
  {
    title: 'Secret',
    key: 'secret_id',
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: 'info' },
        { default: () => secretNameOf(row.secret_id) },
      ),
  },
  {
    title: 'Value',
    key: 'value',
    render: (row) =>
      h(NSpace, { align: 'center', size: 8, wrapItem: false }, () => [
        revealed.value.has(row.id)
          ? h(
              NEllipsis,
              { style: 'max-width: 200px' },
              { default: () => row.value || '—' },
            )
          : h(NText, { depth: 3 }, { default: () => '••••••••' }),
        h(
          NButton,
          { text: true, type: 'primary', size: 'tiny', onClick: () => toggleReveal(row.id) },
          { default: () => (revealed.value.has(row.id) ? 'Hide' : 'Show') },
        ),
      ]),
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
    width: 220,
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
          { onPositiveClick: () => removeItem(row) },
          {
            trigger: () =>
              h(
                NButton,
                { size: 'small', type: 'error', secondary: true },
                { default: () => 'Delete' },
              ),
            default: () => `Delete "${row.key}"?`,
          },
        ),
      ]),
  },
]

onMounted(async () => {
  await searchSecrets()
  if (filters.secretId) await ensureSecret(filters.secretId)
  await fetchItems()
})
</script>

<template>
  <NSpace vertical :size="16">
    <NCard title="Items">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">New item</NButton>
      </template>

      <NFlex align="center" wrap :size="12" style="margin-bottom: 16px">
        <NInput
          v-model:value="filters.search"
          placeholder="Search by key"
          clearable
          style="max-width: 280px"
          @keyup.enter="applyFilters"
          @clear="applyFilters"
        />
        <NSelect
          :value="filters.secretId"
          :options="secretOptions"
          :loading="secretsLoading"
          filterable
          remote
          clearable
          placeholder="Filter by secret"
          style="width: 240px"
          @search="searchSecrets"
          @update:value="onSecretFilterChange"
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
        :row-key="(row: ItemMain) => row.id"
        :pagination="pagination"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NCard>

    <ItemDetailDrawer v-model:show="showDetail" :item-id="detailId" />
    <ItemFormModal
      v-model:show="showForm"
      :item="editing"
      :default-secret-id="filters.secretId"
      @saved="fetchItems"
    />
  </NSpace>
</template>
