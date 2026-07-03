<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NFlex,
  NIcon,
  NPagination,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { Pencil, Plus, Trash } from '@vicons/tabler'

import { deleteApiKey, listApiKeys, updateApiKey } from '@/api/apikey'
import { apiErrorMessage } from '@/api/http'
import { getUser, listUsers } from '@/api/usr'
import type { ApiKeyListReq, ApiKeyMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useAuthStore } from '@/stores/auth'
import { formatDate } from '@/utils/format'

import ApiKeyFormModal from '@/components/apikey/ApiKeyFormModal.vue'

const message = useMessage()
const authStore = useAuthStore()
const { isMobile } = useBreakpoint()

const rows = ref<ApiKeyMain[]>([])
const loading = ref(false)

const filters = reactive<{ active: boolean | null; usrId: string | null }>({
  active: null,
  usrId: null,
})

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

const editing = ref<ApiKeyMain | null>(null)
const showForm = ref(false)

// ── Owner labels (admin sees keys of all users) ────────────

const ownerNames = reactive<Record<string, string>>({})

async function resolveOwners(items: ApiKeyMain[]): Promise<void> {
  if (!authStore.isAdmin) return
  const unknown = [...new Set(items.map((r) => String(r.usr_id)))].filter(
    (id) => !(id in ownerNames),
  )
  await Promise.all(
    unknown.map(async (id) => {
      try {
        const u = await getUser(id)
        ownerNames[id] = u.name ? `${u.name} (${u.username})` : u.username
      } catch {
        ownerNames[id] = `#${id}`
      }
    }),
  )
}

function ownerLabel(row: ApiKeyMain): string {
  return ownerNames[String(row.usr_id)] ?? `#${row.usr_id}`
}

// Owner filter options (admin only, remote search).
const usrOptions = ref<SelectOption[]>([])
const usrLoading = ref(false)

async function searchUsers(query: string): Promise<void> {
  usrLoading.value = true
  try {
    const rep = await listUsers({
      list_params: { page: 0, page_size: 50 },
      ...(query.trim() ? { search: query.trim() } : {}),
    })
    usrOptions.value = (rep.results ?? []).map((u) => ({
      label: u.name ? `${u.name} (${u.username})` : u.username,
      value: String(u.id),
    }))
  } catch {
    // фильтр по владельцу — вспомогательный; ошибку не эскалируем
  } finally {
    usrLoading.value = false
  }
}

// ── Data ───────────────────────────────────────────────────

async function fetchKeys(): Promise<void> {
  loading.value = true
  try {
    const req: ApiKeyListReq = {
      list_params: {
        // API pagination is zero-based; naive-ui's table is 1-based.
        page: pagination.page - 1,
        page_size: pagination.pageSize,
        with_total_count: true,
      },
    }
    if (filters.active !== null) req.active = filters.active
    if (filters.usrId) req.usr_id = filters.usrId

    const rep = await listApiKeys(req)
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
    await resolveOwners(rows.value)
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load API keys'))
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  pagination.page = 1
  void fetchKeys()
}

function onStatusChange(value: string): void {
  filters.active = value === 'all' ? null : value === 'active'
  applyFilters()
}

function onOwnerChange(value: string | null): void {
  filters.usrId = value
  applyFilters()
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchKeys()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchKeys()
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: ApiKeyMain): void {
  editing.value = row
  showForm.value = true
}

// ── Row actions ────────────────────────────────────────────

const togglingId = ref<string | null>(null)

async function toggleActive(row: ApiKeyMain, active: boolean): Promise<void> {
  togglingId.value = row.id
  try {
    await updateApiKey(row.id, { active })
    row.active = active
    message.success(active ? 'API key enabled' : 'API key disabled')
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to update API key'))
  } finally {
    togglingId.value = null
  }
}

async function removeKey(row: ApiKeyMain): Promise<void> {
  try {
    await deleteApiKey(row.id)
    message.success('API key deleted')
    if (rows.value.length === 1 && pagination.page > 1) {
      pagination.page -= 1
    }
    await fetchKeys()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to delete API key'))
  }
}

// ── Table ──────────────────────────────────────────────────

function iconButton(icon: typeof Pencil, tooltip: string, onClick: () => void) {
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
            'aria-label': tooltip,
            onClick,
          },
          { icon: () => h(NIcon, { component: icon }) },
        ),
      default: () => tooltip,
    },
  )
}

function typeTag(row: ApiKeyMain) {
  return row.mcp_only
    ? h(NTag, { type: 'info', size: 'small' }, { default: () => 'MCP only' })
    : h(NTag, { type: 'warning', size: 'small' }, { default: () => 'Full API' })
}

const columns = computed<DataTableColumns<ApiKeyMain>>(() => [
  {
    title: 'Name',
    key: 'name',
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', onClick: () => openEdit(row) },
        { default: () => row.name || '—' },
      ),
  },
  {
    title: 'Key',
    key: 'key_prefix',
    width: 150,
    render: (row) =>
      h(
        NText,
        { code: true },
        { default: () => (row.key_prefix ? `${row.key_prefix}…` : '—') },
      ),
  },
  ...(authStore.isAdmin
    ? [
        {
          title: 'Owner',
          key: 'usr_id',
          render: (row: ApiKeyMain) => ownerLabel(row),
        },
      ]
    : []),
  {
    title: 'Type',
    key: 'mcp_only',
    width: 110,
    render: typeTag,
  },
  {
    title: 'Active',
    key: 'active',
    width: 90,
    render: (row) =>
      h(NSwitch, {
        value: row.active,
        size: 'small',
        loading: togglingId.value === row.id,
        'aria-label': 'Toggle key',
        onUpdateValue: (value: boolean) => void toggleActive(row, value),
      }),
  },
  {
    title: 'Last used',
    key: 'last_used_at',
    width: 170,
    render: (row) => (row.last_used_at ? formatDate(row.last_used_at) : 'Never'),
  },
  {
    title: 'Created',
    key: 'created_at',
    width: 170,
    render: (row) => formatDate(row.created_at),
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 110,
    render: (row) =>
      h(NSpace, { size: 4, wrapItem: false }, () => [
        iconButton(Pencil, 'Edit', () => openEdit(row)),
        h(
          NPopconfirm,
          { onPositiveClick: () => removeKey(row) },
          {
            trigger: () =>
              h(
                NButton,
                {
                  quaternary: true,
                  circle: true,
                  size: 'small',
                  type: 'error',
                  'aria-label': 'Delete API key',
                },
                { icon: () => h(NIcon, { component: Trash }) },
              ),
            default: () => `Delete key "${row.name}"? Clients using it will lose access.`,
          },
        ),
      ]),
  },
])

onMounted(() => {
  void fetchKeys()
  if (authStore.isAdmin) void searchUsers('')
})
</script>

<template>
  <NSpace vertical :size="16">
    <NCard title="API keys" class="stack-header">
      <template #header-extra>
        <NButton type="primary" @click="openCreate">
          <template #icon>
            <NIcon :component="Plus" />
          </template>
          New key
        </NButton>
      </template>

      <NText depth="3" style="display: block; margin-bottom: 16px; font-size: 13px">
        Long-lived credentials for machine clients (MCP agents, CI). The key
        value is shown once at creation; kusec stores only a hash. “MCP only”
        keys are accepted solely by the MCP endpoint, so agents cannot read
        secret values through the main API.
      </NText>

      <NFlex align="center" wrap :size="12" style="margin-bottom: 16px">
        <NSelect
          :default-value="'all'"
          :options="statusOptions"
          style="width: 160px"
          @update:value="onStatusChange"
        />
        <NSelect
          v-if="authStore.isAdmin"
          :value="filters.usrId"
          :options="usrOptions"
          :loading="usrLoading"
          remote
          filterable
          clearable
          placeholder="All owners"
          style="width: 240px"
          @search="searchUsers"
          @update:value="onOwnerChange"
        />
      </NFlex>

      <!-- Mobile: stacked cards + standalone pager (no horizontal scroll). -->
      <template v-if="isMobile">
        <NEmpty
          v-if="!rows.length && !loading"
          description="No API keys"
          style="padding: 24px 0"
        />
        <div class="key-cards">
          <NCard v-for="row in rows" :key="row.id" size="small">
            <div class="key-card__top">
              <NButton text type="primary" @click="openEdit(row)">
                {{ row.name || '—' }}
              </NButton>
              <NSwitch
                :value="row.active"
                size="small"
                :loading="togglingId === row.id"
                aria-label="Toggle key"
                @update:value="(v: boolean) => toggleActive(row, v)"
              />
            </div>
            <NText code style="font-size: 12px">{{ row.key_prefix }}…</NText>
            <NSpace :size="6" style="margin-top: 8px">
              <NTag :type="row.mcp_only ? 'info' : 'warning'" size="small">
                {{ row.mcp_only ? 'MCP only' : 'Full API' }}
              </NTag>
              <NTag v-if="authStore.isAdmin" size="small">{{ ownerLabel(row) }}</NTag>
            </NSpace>
            <NText depth="3" style="display: block; margin-top: 8px; font-size: 12px">
              Last used: {{ row.last_used_at ? formatDate(row.last_used_at) : 'Never' }}
            </NText>
            <template #action>
              <NSpace justify="end" :size="4" :wrap-item="false">
                <NButton
                  quaternary
                  circle
                  size="small"
                  aria-label="Edit API key"
                  @click="openEdit(row)"
                >
                  <template #icon>
                    <NIcon :component="Pencil" />
                  </template>
                </NButton>
                <NPopconfirm @positive-click="removeKey(row)">
                  <template #trigger>
                    <NButton
                      quaternary
                      circle
                      size="small"
                      type="error"
                      aria-label="Delete API key"
                    >
                      <template #icon>
                        <NIcon :component="Trash" />
                      </template>
                    </NButton>
                  </template>
                  Delete key "{{ row.name }}"? Clients using it will lose access.
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
        :row-key="(row: ApiKeyMain) => row.id"
        :pagination="pagination"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NCard>

    <ApiKeyFormModal v-model:show="showForm" :api-key="editing" @saved="fetchKeys" />
  </NSpace>
</template>

<style scoped>
.key-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.key-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}
</style>
