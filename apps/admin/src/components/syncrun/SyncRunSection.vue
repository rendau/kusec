<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NDataTable,
  NEmpty,
  NFlex,
  NPagination,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'

import { apiErrorMessage } from '@/api/http'
import { getSyncRun, listSyncRuns } from '@/api/syncrun'
import type { SyncRunMain, SyncRunObject } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  /** Scope the journal to one application (app workspace tab). */
  appId?: string
}>()

const message = useMessage()
const { isMobile } = useBreakpoint()

const rows = ref<SyncRunMain[]>([])
const loading = ref(false)

const filters = reactive<{ status: string | null; window: string }>({
  status: null,
  window: '7d',
})

const statusOptions: SelectOption[] = [
  { label: 'All statuses', value: 'all' },
  { label: 'Ok', value: 'ok' },
  { label: 'Partial', value: 'partial' },
  { label: 'Error', value: 'error' },
  { label: 'Running', value: 'running' },
]

const windowOptions: SelectOption[] = [
  { label: 'Last 24 hours', value: '24h' },
  { label: 'Last 7 days', value: '7d' },
  { label: 'Last 30 days', value: '30d' },
  { label: 'All time', value: 'all' },
]

const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
})

function windowGte(): string | undefined {
  const hours: Record<string, number> = { '24h': 24, '7d': 168, '30d': 720 }
  const h = hours[filters.window]
  if (!h) return undefined
  return new Date(Date.now() - h * 3600_000).toISOString()
}

async function fetchRuns(): Promise<void> {
  loading.value = true
  const gte = windowGte()
  try {
    const rep = await listSyncRuns({
      list_params: {
        // API pagination is zero-based; naive-ui is 1-based.
        page: pagination.page - 1,
        page_size: pagination.pageSize,
        with_total_count: true,
      },
      ...(props.appId ? { app_id: props.appId } : {}),
      ...(filters.status ? { status: filters.status } : {}),
      ...(gte ? { started_at_gte: gte } : {}),
    })
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load sync runs'))
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  pagination.page = 1
  void fetchRuns()
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchRuns()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchRuns()
}

// ── Expanded run details (objects are fetched lazily) ──────

const objectsByRun = reactive<Record<string, SyncRunObject[] | 'loading' | 'error'>>({})

async function loadObjects(runId: string): Promise<void> {
  if (objectsByRun[runId] && objectsByRun[runId] !== 'error') return
  objectsByRun[runId] = 'loading'
  try {
    const run = await getSyncRun(runId)
    objectsByRun[runId] = run.objects ?? []
  } catch {
    objectsByRun[runId] = 'error'
  }
}

function onExpandedChange(keys: Array<string | number>): void {
  for (const key of keys) void loadObjects(String(key))
}

const statusTagType: Record<string, 'success' | 'warning' | 'error' | 'info'> = {
  ok: 'success',
  partial: 'warning',
  error: 'error',
  running: 'info',
}

function statusTag(row: SyncRunMain) {
  return h(
    NTag,
    { size: 'small', type: statusTagType[row.status] ?? 'info' },
    { default: () => row.status },
  )
}

const opTagType: Record<string, 'success' | 'info' | 'error' | 'warning' | 'default'> = {
  created: 'success',
  updated: 'info',
  deleted: 'error',
  unchanged: 'default',
  error: 'error',
}

function renderObjects(row: SyncRunMain) {
  const state = objectsByRun[row.id]
  if (state === 'loading' || state === undefined) {
    return h(NSpin, { size: 'small' })
  }
  if (state === 'error') {
    return h(NText, { depth: 3 }, { default: () => 'Failed to load run objects.' })
  }
  if (!state.length) {
    return h(NText, { depth: 3 }, { default: () => 'No objects touched by this run.' })
  }
  return h(
    NSpace,
    { vertical: true, size: 4 },
    {
      default: () =>
        state.map((obj) =>
          h(NSpace, { size: 6, wrapItem: false, align: 'center' }, {
            default: () => [
              h(
                NTag,
                { size: 'tiny', type: opTagType[obj.op] ?? 'default' },
                { default: () => obj.op },
              ),
              h(
                NText,
                { code: true, style: 'font-size: 12px; word-break: break-all' },
                {
                  default: () =>
                    `${obj.namespace}/${obj.kube_name} (${obj.kube_kind})` +
                    (obj.changed_keys.length ? ` · keys: ${obj.changed_keys.join(', ')}` : '') +
                    (obj.error ? ` · ${obj.error}` : ''),
                },
              ),
            ],
          }),
        ),
    },
  )
}

/** "app scope" cell: one app or the whole accessible set. */
function scopeLabel(row: SyncRunMain): string {
  return row.app_id ? 'single app' : 'all apps'
}

function durationLabel(row: SyncRunMain): string {
  const ms = Number(row.duration_ms ?? 0)
  if (!ms) return '—'
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`
}

const columns = computed<DataTableColumns<SyncRunMain>>(() => [
  {
    type: 'expand',
    renderExpand: renderObjects,
  },
  {
    title: 'Started',
    key: 'started_at',
    width: 170,
    render: (row) => formatDate(row.started_at),
  },
  {
    title: 'Status',
    key: 'status',
    width: 100,
    render: statusTag,
  },
  {
    title: 'Actor',
    key: 'actor_name',
    minWidth: 140,
    ellipsis: { tooltip: true },
    render: (row) =>
      h(NSpace, { size: 6, wrapItem: false, align: 'center' }, {
        default: () => [
          row.actor_name || '—',
          h(NTag, { size: 'tiny', bordered: false }, { default: () => row.source || '—' }),
        ],
      }),
  },
  ...(props.appId
    ? []
    : [
        {
          title: 'Scope',
          key: 'scope',
          width: 100,
          render: (row: SyncRunMain) => scopeLabel(row),
        },
      ]),
  {
    title: 'Duration',
    key: 'duration_ms',
    width: 100,
    render: durationLabel,
  },
  {
    title: 'Error',
    key: 'error',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row) => row.error || '—',
  },
])

onMounted(() => {
  void fetchRuns()
})

defineExpose({ refresh: fetchRuns })
</script>

<template>
  <NSpace vertical :size="12">
    <NFlex :size="8" :wrap="true" align="center">
      <NSelect
        :value="filters.window"
        :options="windowOptions"
        size="small"
        style="width: 150px"
        @update:value="
          (v: string) => {
            filters.window = v
            applyFilters()
          }
        "
      />
      <NSelect
        :value="filters.status ?? 'all'"
        :options="statusOptions"
        size="small"
        style="width: 140px"
        @update:value="
          (v: string) => {
            filters.status = v === 'all' ? null : v
            applyFilters()
          }
        "
      />
    </NFlex>

    <!-- Mobile: card stack instead of a dense table. -->
    <template v-if="isMobile">
      <NEmpty v-if="!loading && !rows.length" description="No sync runs" />
      <div v-else class="run-cards">
        <div v-for="row in rows" :key="row.id" class="run-card">
          <NSpace align="center" :size="6" :wrap-item="false">
            <component :is="() => statusTag(row)" />
            <NText style="font-size: 13px">{{ formatDate(row.started_at) }}</NText>
          </NSpace>
          <NText depth="3" style="font-size: 12px; display: block; margin-top: 4px">
            {{ row.actor_name || '—' }} · {{ row.source }} · {{ durationLabel(row) }}
            <template v-if="!props.appId"> · {{ scopeLabel(row) }}</template>
          </NText>
          <NText
            v-if="row.error"
            type="error"
            style="font-size: 12px; display: block; margin-top: 4px"
          >
            {{ row.error }}
          </NText>
        </div>
      </div>
    </template>

    <NDataTable
      v-else
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(row: SyncRunMain) => row.id"
      size="small"
      :bordered="false"
      @update:expanded-row-keys="onExpandedChange"
    />

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
  </NSpace>
</template>

<style scoped>
.run-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.run-card {
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.2));
  border-radius: 8px;
  padding: 10px 12px;
}
</style>
