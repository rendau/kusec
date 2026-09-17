<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NDataTable,
  NEmpty,
  NFlex,
  NInput,
  NPagination,
  NSelect,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'

import { listAudit } from '@/api/audit'
import { apiErrorMessage } from '@/api/http'
import type { AuditChange, AuditMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  /** Scope the feed to one application (app workspace tab). */
  appId?: string
}>()

const message = useMessage()
const { isMobile } = useBreakpoint()

const rows = ref<AuditMain[]>([])
const loading = ref(false)

const filters = reactive<{
  entityType: string | null
  action: string | null
  key: string
  kubeName: string
  namespace: string
  window: string
}>({
  entityType: null,
  action: null,
  key: '',
  kubeName: '',
  namespace: '',
  window: '24h',
})

const entityTypeOptions: SelectOption[] = [
  { label: 'All entities', value: 'all' },
  { label: 'Applications', value: 'app' },
  { label: 'Secrets', value: 'secret' },
  { label: 'Secret items', value: 'item' },
  { label: 'Config maps', value: 'configmap' },
  { label: 'Config items', value: 'config_item' },
  { label: 'API keys', value: 'api_key' },
  { label: 'Users', value: 'usr' },
  { label: 'Sync runs', value: 'sync_run' },
]

const actionOptions: SelectOption[] = [
  { label: 'All actions', value: 'all' },
  { label: 'Create', value: 'create' },
  { label: 'Update', value: 'update' },
  { label: 'Delete', value: 'delete' },
  { label: 'Activate', value: 'activate' },
  { label: 'Deactivate', value: 'deactivate' },
  { label: 'Sync', value: 'sync' },
  { label: 'Import', value: 'import' },
]

const windowOptions: SelectOption[] = [
  { label: 'Last hour', value: '1h' },
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
  pageSizes: [10, 20, 50, 100],
})

function windowGte(): string | undefined {
  const hours: Record<string, number> = { '1h': 1, '24h': 24, '7d': 168, '30d': 720 }
  const h = hours[filters.window]
  if (!h) return undefined
  return new Date(Date.now() - h * 3600_000).toISOString()
}

async function fetchAudit(): Promise<void> {
  loading.value = true
  const gte = windowGte()
  try {
    const rep = await listAudit({
      list_params: {
        // API pagination is zero-based; naive-ui is 1-based.
        page: pagination.page - 1,
        page_size: pagination.pageSize,
        with_total_count: true,
      },
      ...(props.appId ? { app_id: props.appId } : {}),
      ...(filters.entityType ? { entity_type: filters.entityType } : {}),
      ...(filters.action ? { action: filters.action } : {}),
      ...(filters.key.trim() ? { key: filters.key.trim() } : {}),
      ...(filters.kubeName.trim() ? { kube_name: filters.kubeName.trim() } : {}),
      ...(filters.namespace.trim() ? { namespace: filters.namespace.trim() } : {}),
      ...(gte ? { created_at_gte: gte } : {}),
    })
    rows.value = rep.results ?? []
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0)
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load the audit log'))
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  pagination.page = 1
  void fetchAudit()
}

function onPageChange(page: number): void {
  pagination.page = page
  void fetchAudit()
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size
  pagination.page = 1
  void fetchAudit()
}

// ── Rendering ──────────────────────────────────────────────

const actionTagType: Record<string, 'success' | 'info' | 'error' | 'warning' | 'default'> = {
  create: 'success',
  update: 'info',
  delete: 'error',
  activate: 'success',
  deactivate: 'warning',
  sync: 'default',
  import: 'info',
}

function actionTag(row: AuditMain) {
  return h(
    NTag,
    { size: 'small', type: actionTagType[row.action] ?? 'default' },
    { default: () => row.action },
  )
}

/** "item PG_PASSWORD @ kusec-caravan-main" style object label. */
function objectLabel(row: AuditMain): string {
  const parts: string[] = [row.entity_type]
  if (row.key) parts.push(row.key)
  else if (row.entity_type === 'usr' || row.entity_type === 'api_key') parts.push(row.entity_id)
  const target = parts.join(' ')
  return row.kube_name ? `${target} @ ${row.kube_name}` : target
}

/** One change line: masked values show fingerprints, plain ones old → new. */
function changeLabel(change: AuditChange): string {
  const size = (v: number | string | null | undefined) => (v == null ? '' : `${v} B`)
  if (change.old_hash || change.new_hash) {
    const from = change.old_hash ? `${change.old_hash} (${size(change.old_size)})` : '—'
    const to = change.new_hash ? `${change.new_hash} (${size(change.new_size)})` : '—'
    const note = change.truncated ? ' · value too large, fingerprint only' : ' · value hidden'
    return `${change.field}: ${from} → ${to}${note}`
  }
  const from = change.old == null ? '—' : change.old === '' ? '""' : change.old
  const to = change.new == null ? '—' : change.new === '' ? '""' : change.new
  return `${change.field}: ${from} → ${to}`
}

function renderChanges(row: AuditMain) {
  if (!row.changes.length) {
    return h(NText, { depth: 3 }, { default: () => 'No field changes recorded.' })
  }
  return h(
    NSpace,
    { vertical: true, size: 4 },
    {
      default: () =>
        row.changes.map((change) =>
          h(
            NText,
            { code: true, style: 'font-size: 12px; word-break: break-all' },
            { default: () => changeLabel(change) },
          ),
        ),
    },
  )
}

const columns = computed<DataTableColumns<AuditMain>>(() => [
  {
    type: 'expand',
    renderExpand: renderChanges,
  },
  {
    title: 'When',
    key: 'created_at',
    width: 170,
    render: (row) => formatDate(row.created_at),
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
  {
    title: 'Action',
    key: 'action',
    width: 110,
    render: actionTag,
  },
  {
    title: 'Object',
    key: 'object',
    minWidth: 220,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { code: true }, { default: () => objectLabel(row) }),
  },
  ...(props.appId
    ? []
    : [
        {
          title: 'Namespace',
          key: 'namespace',
          width: 140,
          ellipsis: { tooltip: true },
          render: (row: AuditMain) => row.namespace || '—',
        },
      ]),
  {
    title: 'Changes',
    key: 'changes',
    width: 90,
    render: (row) => String(row.changes.length || '—'),
  },
])

onMounted(() => {
  void fetchAudit()
})

defineExpose({ refresh: fetchAudit })
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
        :value="filters.entityType ?? 'all'"
        :options="entityTypeOptions"
        size="small"
        style="width: 150px"
        @update:value="
          (v: string) => {
            filters.entityType = v === 'all' ? null : v
            applyFilters()
          }
        "
      />
      <NSelect
        :value="filters.action ?? 'all'"
        :options="actionOptions"
        size="small"
        style="width: 140px"
        @update:value="
          (v: string) => {
            filters.action = v === 'all' ? null : v
            applyFilters()
          }
        "
      />
      <NInput
        v-model:value="filters.key"
        size="small"
        placeholder="Key (exact)"
        clearable
        style="width: 160px"
        @keyup.enter="applyFilters"
        @clear="applyFilters"
      />
      <template v-if="!props.appId">
        <NInput
          v-model:value="filters.namespace"
          size="small"
          placeholder="Namespace"
          clearable
          style="width: 140px"
          @keyup.enter="applyFilters"
          @clear="applyFilters"
        />
        <NInput
          v-model:value="filters.kubeName"
          size="small"
          placeholder="k8s object name"
          clearable
          style="width: 180px"
          @keyup.enter="applyFilters"
          @clear="applyFilters"
        />
      </template>
    </NFlex>

    <!-- Mobile: card stack instead of a dense table. -->
    <template v-if="isMobile">
      <NEmpty v-if="!loading && !rows.length" description="No audit entries" />
      <div v-else class="audit-cards">
        <div v-for="row in rows" :key="String(row.id)" class="audit-card">
          <NSpace align="center" :size="6" :wrap-item="false">
            <component :is="() => actionTag(row)" />
            <NText code style="font-size: 12px; word-break: break-all">
              {{ objectLabel(row) }}
            </NText>
          </NSpace>
          <NText depth="3" style="font-size: 12px; display: block; margin-top: 4px">
            {{ row.actor_name || '—' }} · {{ row.source }} · {{ formatDate(row.created_at) }}
          </NText>
          <component :is="() => renderChanges(row)" style="margin-top: 6px" />
        </div>
      </div>
    </template>

    <NDataTable
      v-else
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(row: AuditMain) => String(row.id)"
      size="small"
      :bordered="false"
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
.audit-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.audit-card {
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.2));
  border-radius: 8px;
  padding: 10px 12px;
}
</style>
