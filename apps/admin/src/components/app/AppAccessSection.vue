<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NFlex,
  NPagination,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

import { apiErrorMessage } from '@/api/http'
import { listUsers } from '@/api/usr'
import type { UsrMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'

import UsrDetailDrawer from '@/components/usr/UsrDetailDrawer.vue'

const props = defineProps<{
  appId: string
}>()

const message = useMessage()
const { isMobile } = useBreakpoint()

const rows = ref<UsrMain[]>([])
const loading = ref(false)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
})

const detailId = ref<number | string | null>(null)
const showDetail = ref(false)

async function fetchUsers(): Promise<void> {
  loading.value = true
  try {
    const rep = await listUsers({
      list_params: {
        // API pagination is zero-based; naive-ui is 1-based.
        page: pagination.page - 1,
        page_size: pagination.pageSize,
        with_total_count: true,
      },
      is_admin: false,
      app_id: props.appId,
    })
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load users'))
  } finally {
    loading.value = false
  }
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

function nameButton(row: UsrMain) {
  return h(
    NButton,
    { text: true, type: 'primary', onClick: () => openDetail(row) },
    { default: () => row.name || '—' },
  )
}

// Empty app_ids means no restriction: the user reaches every application.
function scopeTag(row: UsrMain) {
  const count = row.app_ids?.length ?? 0
  return h(
    NTag,
    { size: 'small', type: count === 0 ? 'warning' : 'default' },
    { default: () => (count === 0 ? 'All apps' : `${count} app${count === 1 ? '' : 's'}`) },
  )
}

function statusTag(row: UsrMain) {
  return h(
    NTag,
    { size: 'small', type: row.active ? 'success' : 'default' },
    { default: () => (row.active ? 'Active' : 'Inactive') },
  )
}

const columns: DataTableColumns<UsrMain> = [
  { title: 'Name', key: 'name', render: nameButton },
  { title: 'Username', key: 'username' },
  { title: 'Access', key: 'app_ids', width: 110, render: scopeTag },
  { title: 'Status', key: 'active', width: 110, render: statusTag },
]

onMounted(() => {
  void fetchUsers()
})
</script>

<template>
  <NSpace vertical :size="12">
    <NText depth="3" style="font-size: 12px">
      Non-admin users who can open this application. Administrators are not listed: they
      always have access. “All apps” means the user has no application restriction.
    </NText>

    <!-- Mobile: card stack instead of a dense table. -->
    <template v-if="isMobile">
      <NEmpty v-if="!loading && !rows.length" description="No non-admin users have access" />
      <div v-else class="access-cards">
        <div v-for="row in rows" :key="row.id" class="access-card">
          <component :is="() => nameButton(row)" />
          <NText depth="3" style="font-size: 12px; display: block; margin: 4px 0">
            {{ row.username }}
          </NText>
          <NSpace :size="6" :wrap-item="false">
            <component :is="() => scopeTag(row)" />
            <component :is="() => statusTag(row)" />
          </NSpace>
        </div>
      </div>
    </template>

    <NDataTable
      v-else
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(row: UsrMain) => row.id"
      size="small"
      :bordered="false"
    >
      <template #empty>
        <NEmpty description="No non-admin users have access" />
      </template>
    </NDataTable>

    <NFlex justify="end">
      <NPagination
        :page="pagination.page"
        :page-size="pagination.pageSize"
        :item-count="pagination.itemCount"
        :page-sizes="pagination.pageSizes"
        :show-size-picker="pagination.showSizePicker && !isMobile"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NFlex>

    <UsrDetailDrawer v-model:show="showDetail" :user-id="detailId" />
  </NSpace>
</template>

<style scoped>
.access-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.access-card {
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.2));
  border-radius: 8px;
  padding: 10px 12px;
}
</style>
