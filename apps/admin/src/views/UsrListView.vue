<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NFlex,
  NIcon,
  NInput,
  NPagination,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { InfoCircle, Pencil, Plus, ShieldOff, Trash } from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { deleteUser, listUsers, resetUserTotp } from '@/api/usr'
import type { UsrListReq, UsrMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useAuthStore } from '@/stores/auth'

import UsrDetailDrawer from '@/components/usr/UsrDetailDrawer.vue'
import UsrFormModal from '@/components/usr/UsrFormModal.vue'

const SEARCH_DEBOUNCE_MS = 350

const message = useMessage()
const authStore = useAuthStore()
const { isMobile } = useBreakpoint()

function accessLabel(row: UsrMain): string {
  const count = row.is_admin ? 0 : (row.app_ids?.length ?? 0)
  return count === 0 ? 'All apps' : `${count} app${count === 1 ? '' : 's'}`
}

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

// Admin-reset: clear another user's 2FA when they lost their authenticator.
async function resetTotp(row: UsrMain): Promise<void> {
  try {
    await resetUserTotp(row.id)
    message.success(`2FA reset for "${row.username}"`)
    await fetchUsers()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to reset 2FA'))
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
    title: 'Access',
    key: 'app_ids',
    width: 130,
    // Admins and users with no scope reach all apps; otherwise it's limited.
    render: (row) => {
      const count = row.is_admin ? 0 : (row.app_ids?.length ?? 0)
      return count === 0
        ? h(NTag, { size: 'small' }, { default: () => 'All apps' })
        : h(
            NTag,
            { type: 'info', size: 'small' },
            { default: () => `${count} app${count === 1 ? '' : 's'}` },
          )
    },
  },
  {
    title: '2FA',
    key: 'totp_enabled',
    width: 90,
    render: (row) =>
      h(
        NTag,
        { type: row.totp_enabled ? 'success' : 'default', size: 'small' },
        { default: () => (row.totp_enabled ? 'On' : 'Off') },
      ),
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 180,
    render: (row) =>
      h(NSpace, { size: 4, wrapItem: false }, () => [
        iconButton(InfoCircle, 'Details', () => openDetail(row)),
        iconButton(Pencil, 'Edit', () => openEdit(row)),
        row.totp_enabled
          ? h(
              NPopconfirm,
              { onPositiveClick: () => resetTotp(row) },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      quaternary: true,
                      circle: true,
                      size: 'small',
                      type: 'warning',
                      'aria-label': 'Reset 2FA',
                    },
                    { icon: () => h(NIcon, { component: ShieldOff }) },
                  ),
                default: () => `Reset 2FA for "${row.username}"?`,
              },
            )
          : null,
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
    <NCard title="Users" class="stack-header">
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

      <!-- Mobile: stacked cards + standalone pager (no horizontal scroll). -->
      <template v-if="isMobile">
        <NEmpty
          v-if="!rows.length && !loading"
          description="No users found"
          style="padding: 24px 0"
        />
        <div class="usr-cards">
          <NCard v-for="row in rows" :key="row.id" size="small" class="usr-card">
            <div class="usr-card__top">
              <NButton text type="primary" @click="openDetail(row)">
                {{ row.name || '—' }}
              </NButton>
              <NTag :type="row.active ? 'success' : 'default'" size="small">
                {{ row.active ? 'Active' : 'Inactive' }}
              </NTag>
            </div>
            <NText depth="3" style="font-size: 13px">{{ row.username }}</NText>
            <NSpace :size="6" style="margin-top: 8px">
              <NTag :type="row.is_admin ? 'success' : 'default'" size="small">
                {{ row.is_admin ? 'Administrator' : 'User' }}
              </NTag>
              <NTag
                :type="accessLabel(row) === 'All apps' ? 'default' : 'info'"
                size="small"
              >
                {{ accessLabel(row) }}
              </NTag>
              <NTag v-if="row.totp_enabled" type="success" size="small"> 2FA </NTag>
            </NSpace>
            <template #action>
              <NSpace justify="end" :size="4" :wrap-item="false">
                <NButton
                  quaternary
                  circle
                  size="small"
                  aria-label="Details"
                  @click="openDetail(row)"
                >
                  <template #icon>
                    <NIcon :component="InfoCircle" />
                  </template>
                </NButton>
                <NButton
                  quaternary
                  circle
                  size="small"
                  aria-label="Edit user"
                  @click="openEdit(row)"
                >
                  <template #icon>
                    <NIcon :component="Pencil" />
                  </template>
                </NButton>
                <NPopconfirm v-if="row.totp_enabled" @positive-click="resetTotp(row)">
                  <template #trigger>
                    <NButton
                      quaternary
                      circle
                      size="small"
                      type="warning"
                      aria-label="Reset 2FA"
                    >
                      <template #icon>
                        <NIcon :component="ShieldOff" />
                      </template>
                    </NButton>
                  </template>
                  Reset 2FA for "{{ row.username }}"?
                </NPopconfirm>
                <NButton
                  v-if="isSelf(row)"
                  quaternary
                  circle
                  size="small"
                  type="error"
                  disabled
                  aria-label="You cannot delete yourself"
                >
                  <template #icon>
                    <NIcon :component="Trash" />
                  </template>
                </NButton>
                <NPopconfirm v-else @positive-click="removeUser(row)">
                  <template #trigger>
                    <NButton
                      quaternary
                      circle
                      size="small"
                      type="error"
                      aria-label="Delete user"
                    >
                      <template #icon>
                        <NIcon :component="Trash" />
                      </template>
                    </NButton>
                  </template>
                  Delete "{{ row.username }}"?
                </NPopconfirm>
              </NSpace>
            </template>
          </NCard>
        </div>
        <NFlex justify="center" style="margin-top: 16px">
          <NPagination
            :page="pagination.page"
            :page-size="pagination.pageSize"
            :item-count="pagination.itemCount"
            @update:page="onPageChange"
          />
        </NFlex>
      </template>

      <NDataTable
        v-else
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

<style scoped>
.usr-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.usr-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}
</style>
