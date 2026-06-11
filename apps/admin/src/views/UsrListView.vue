<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NFlex,
  NIcon,
  NInput,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { InfoCircle, Pencil, Plus, Trash } from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { deleteUser, listUsers } from '@/api/usr'
import type { UsrListReq, UsrMain } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

import UsrDetailDrawer from '@/components/usr/UsrDetailDrawer.vue'
import UsrFormModal from '@/components/usr/UsrFormModal.vue'

const SEARCH_DEBOUNCE_MS = 350

const message = useMessage()
const authStore = useAuthStore()

const currentUserId = computed(() =>
  authStore.profile ? String(authStore.profile.id) : null,
)

const rows = ref<UsrMain[]>([])
const loading = ref(false)

const filters = reactive<{
  search: string
  active: boolean | null
  isAdmin: boolean | null
}>({
  search: '',
  active: null,
  isAdmin: null,
})

const roleOptions: SelectOption[] = [
  { label: 'All roles', value: 'all' },
  { label: 'Administrators', value: 'admin' },
  { label: 'Users', value: 'user' },
]

const statusOptions: SelectOption[] = [
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

const detailId = ref<number | string | null>(null)
const showDetail = ref(false)

const editing = ref<UsrMain | null>(null)
const showForm = ref(false)

async function fetchUsers(): Promise<void> {
  loading.value = true
  try {
    const req: UsrListReq = {
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
    if (filters.isAdmin !== null) req.is_admin = filters.isAdmin

    const rep = await listUsers(req)
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load users'))
  } finally {
    loading.value = false
  }
}

let searchTimer: number | undefined

function applyFilters(): void {
  window.clearTimeout(searchTimer)
  pagination.page = 1
  void fetchUsers()
}

// Search-as-you-type with a small debounce (Enter still applies instantly).
watch(
  () => filters.search,
  () => {
    window.clearTimeout(searchTimer)
    searchTimer = window.setTimeout(applyFilters, SEARCH_DEBOUNCE_MS)
  },
)

onBeforeUnmount(() => {
  window.clearTimeout(searchTimer)
})

function onRoleChange(value: string): void {
  filters.isAdmin = value === 'all' ? null : value === 'admin'
  applyFilters()
}

function onStatusChange(value: string): void {
  filters.active = value === 'all' ? null : value === 'active'
  applyFilters()
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchUsers()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchUsers()
}

function openDetail(row: UsrMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: UsrMain): void {
  editing.value = row
  showForm.value = true
}

function isSelf(row: UsrMain): boolean {
  return currentUserId.value !== null && String(row.id) === currentUserId.value
}

async function removeUser(row: UsrMain): Promise<void> {
  try {
    await deleteUser(row.id)
    message.success('User deleted')
    if (rows.value.length === 1 && pagination.page > 1) {
      pagination.page -= 1
    }
    await fetchUsers()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to delete user'))
  }
}

function iconButton(
  icon: typeof Pencil,
  tooltip: string,
  onClick: () => void,
  options: { type?: 'error'; disabled?: boolean } = {},
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
            ...(options.type ? { type: options.type } : {}),
            ...(options.disabled != null ? { disabled: options.disabled } : {}),
            'aria-label': tooltip,
            onClick,
          },
          { icon: () => h(NIcon, { component: icon }) },
        ),
      default: () => tooltip,
    },
  )
}

const columns: DataTableColumns<UsrMain> = [
  {
    title: 'Name',
    key: 'name',
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', onClick: () => openDetail(row) },
        { default: () => row.name || '—' },
      ),
  },
  { title: 'Username', key: 'username' },
  {
    title: 'Role',
    key: 'is_admin',
    width: 150,
    render: (row) =>
      h(
        NTag,
        { type: row.is_admin ? 'success' : 'default', size: 'small' },
        { default: () => (row.is_admin ? 'Administrator' : 'User') },
      ),
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
        isSelf(row)
          ? iconButton(Trash, 'You cannot delete yourself', () => {}, {
              type: 'error',
              disabled: true,
            })
          : h(
              NPopconfirm,
              { onPositiveClick: () => removeUser(row) },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      quaternary: true,
                      circle: true,
                      size: 'small',
                      type: 'error',
                      'aria-label': 'Delete user',
                    },
                    { icon: () => h(NIcon, { component: Trash }) },
                  ),
                default: () => `Delete "${row.username}"?`,
              },
            ),
      ]),
  },
]

onMounted(fetchUsers)
</script>

<template>
  <NSpace vertical :size="16">
    <NCard title="Users">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">
          <template #icon>
            <NIcon :component="Plus" />
          </template>
          New user
        </NButton>
      </template>

      <NFlex align="center" wrap :size="12" style="margin-bottom: 16px">
        <NInput
          v-model:value="filters.search"
          placeholder="Search by name or username"
          clearable
          style="max-width: 280px"
          @keyup.enter="applyFilters"
        />
        <NSelect
          :default-value="'all'"
          :options="roleOptions"
          style="width: 180px"
          @update:value="onRoleChange"
        />
        <NSelect
          :default-value="'all'"
          :options="statusOptions"
          style="width: 160px"
          @update:value="onStatusChange"
        />
      </NFlex>

      <NDataTable
        remote
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: UsrMain) => row.id"
        :pagination="pagination"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NCard>

    <UsrDetailDrawer v-model:show="showDetail" :user-id="detailId" />
    <UsrFormModal v-model:show="showForm" :user="editing" @saved="fetchUsers" />
  </NSpace>
</template>
