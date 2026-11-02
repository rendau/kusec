<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NCheckbox,
  NDataTable,
  NEmpty,
  NFlex,
  NIcon,
  NInput,
  NPagination,
  NSelect,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, DataTableRowKey } from 'naive-ui'
import { Search, Trash } from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { listInactive } from '@/api/inactive'
import type { InactiveElement, InactiveKind } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useAppsStore } from '@/stores/apps'
import { formatDate } from '@/utils/format'
import { inactiveKindLabel, inactiveKindOptions } from '@/utils/inactive'

import ConfigItemDetailDrawer from '@/components/configitem/ConfigItemDetailDrawer.vue'
import ConfigMapDetailDrawer from '@/components/configmap/ConfigMapDetailDrawer.vue'
import InactiveDeleteModal from '@/components/inactive/InactiveDeleteModal.vue'
import ItemDetailDrawer from '@/components/item/ItemDetailDrawer.vue'
import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { isMobile } = useBreakpoint()
const appsStore = useAppsStore()

const rows = ref<InactiveElement[]>([])
const loading = ref(false)

// The application filter lives in the URL, so the button of an application
// workspace can link straight to its own elements.
const appId = computed(() => (typeof route.query.app === 'string' ? route.query.app : null))

const appOptions = computed(() =>
  appsStore.apps.map((app) => ({ label: app.name, value: app.id })),
)

function setApp(value: string | null): void {
  void router.replace({ name: 'inactive', query: value ? { app: value } : {} })
}

const kind = ref<InactiveKind | null>(null)
const search = ref('')

function rowKey(row: InactiveElement): string {
  return `${row.kind}:${row.id}`
}

// The whole list is on the client, so filters and paging need no requests.
const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(
    (r) =>
      (!kind.value || r.kind === kind.value) &&
      (!q ||
        [r.name, r.parent_name, r.app_name, r.description].some((v) =>
          v.toLowerCase().includes(q),
        )),
  )
})

const page = ref(1)
const pageSize = ref(20)
const pagedRows = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)

watch([filtered, pageSize], () => {
  page.value = 1
})

// Only rows that pass the filters count: a hidden row must not be deleted
// because it was ticked before the filter changed.
const checkedKeys = ref<DataTableRowKey[]>([])
const selected = computed(() => {
  const keys = new Set(checkedKeys.value)
  return filtered.value.filter((r) => keys.has(rowKey(r)))
})
const allSelected = computed(
  () => filtered.value.length > 0 && selected.value.length === filtered.value.length,
)

function toggleSelectAll(): void {
  checkedKeys.value = allSelected.value ? [] : filtered.value.map(rowKey)
}

function toggleRow(row: InactiveElement, checked: boolean): void {
  const key = rowKey(row)
  checkedKeys.value = checked
    ? [...checkedKeys.value, key]
    : checkedKeys.value.filter((k) => k !== key)
}

async function fetchRows(): Promise<void> {
  loading.value = true
  try {
    const rep = await listInactive(appId.value)
    rows.value = rep.results ?? []
    checkedKeys.value = []
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load inactive elements'))
  } finally {
    loading.value = false
  }
}

const showDelete = ref(false)
const toDelete = ref<InactiveElement[]>([])

function openDelete(elements: InactiveElement[]): void {
  toDelete.value = elements
  showDelete.value = true
}

async function onDeleted(): Promise<void> {
  if (toDelete.value.some((e) => e.kind === 'app')) {
    await appsStore.refresh()
    if (appId.value && !appsStore.getById(appId.value)) {
      setApp(null)
      return
    }
  }
  await fetchRows()
}

// Detail cards are the existing drawers, opened from the name of a row.
const detail = ref<{ kind: InactiveKind; id: string } | null>(null)
const showDetail = ref(false)

function detailId(forKind: InactiveKind): string | null {
  return detail.value?.kind === forKind ? detail.value.id : null
}

function detailShown(forKind: InactiveKind): boolean {
  return showDetail.value && detail.value?.kind === forKind
}

function openApp(id: string): void {
  void router.push({ name: 'app-workspace', params: { id } })
}

function openDetail(row: InactiveElement): void {
  if (row.kind === 'app') {
    openApp(row.id)
    return
  }
  detail.value = { kind: row.kind, id: row.id }
  showDetail.value = true
}

function nameCell(row: InactiveElement) {
  return h('div', { class: 'name-cell' }, [
    h(
      NButton,
      { text: true, type: 'primary', class: 'name-cell__name', onClick: () => openDetail(row) },
      { default: () => row.name },
    ),
    row.parent_name
      ? h(NText, { depth: 3, class: 'name-cell__sub' }, { default: () => `in ${row.parent_name}` })
      : null,
    row.description
      ? h(NText, { depth: 3, class: 'name-cell__sub' }, { default: () => row.description })
      : null,
  ])
}

function kindTag(row: InactiveElement) {
  return h(NTag, { size: 'small', bordered: false }, { default: () => inactiveKindLabel(row.kind) })
}

function appLink(row: InactiveElement) {
  return h(
    NButton,
    { text: true, onClick: () => openApp(row.app_id) },
    { default: () => row.app_name },
  )
}

function contains(row: InactiveElement): string {
  const count = Number(row.child_count)
  if (!count) return '—'
  const noun = row.kind === 'app' ? 'object' : 'item'
  return `${count} ${noun}${count === 1 ? '' : 's'}`
}

function deleteButton(row: InactiveElement) {
  return h(
    NButton,
    { size: 'small', type: 'error', secondary: true, onClick: () => openDelete([row]) },
    { default: () => 'Delete' },
  )
}

const columns = computed<DataTableColumns<InactiveElement>>(() => [
  { type: 'selection' },
  { title: 'Name', key: 'name', render: nameCell },
  { title: 'Type', key: 'kind', width: 120, render: kindTag },
  ...(appId.value ? [] : [{ title: 'Application', key: 'app_name', render: appLink }]),
  { title: 'Contains', key: 'child_count', width: 100, render: contains },
  {
    title: 'Updated',
    key: 'updated_at',
    width: 170,
    render: (row: InactiveElement) => formatDate(row.updated_at),
  },
  { title: '', key: 'actions', width: 90, align: 'right', render: deleteButton },
])

watch(appId, () => void fetchRows())

onMounted(() => {
  void appsStore.ensureLoaded()
  void fetchRows()
})
</script>

<template>
  <NCard title="Inactive elements">
    <NSpace vertical :size="12">
      <NText depth="3" style="font-size: 12px">
        Everything that is switched off: it does not reach the cluster, but stays in kusec until
        deleted.
      </NText>

      <div class="filters">
        <NSelect
          :value="appId"
          :options="appOptions"
          placeholder="All applications"
          filterable
          clearable
          class="filters__select"
          @update:value="setApp"
        />
        <NSelect
          v-model:value="kind"
          :options="inactiveKindOptions"
          placeholder="All types"
          clearable
          class="filters__select"
        />
        <NInput v-model:value="search" placeholder="Search" clearable class="filters__search">
          <template #prefix>
            <NIcon :component="Search" />
          </template>
        </NInput>
        <div class="filters__actions">
          <NButton :disabled="!filtered.length" @click="toggleSelectAll">
            {{ allSelected ? 'Clear selection' : `Select all (${filtered.length})` }}
          </NButton>
          <NButton
            type="error"
            :disabled="!selected.length"
            @click="openDelete(selected)"
          >
            <template #icon>
              <NIcon :component="Trash" />
            </template>
            Delete selected ({{ selected.length }})
          </NButton>
        </div>
      </div>

      <!-- Mobile: card stack instead of a dense table. -->
      <template v-if="isMobile">
        <NEmpty v-if="!loading && !filtered.length" description="No inactive elements" />
        <div v-else class="inactive-cards">
          <div v-for="row in pagedRows" :key="rowKey(row)" class="inactive-card">
            <div class="inactive-card__head">
              <NCheckbox
                :checked="checkedKeys.includes(rowKey(row))"
                @update:checked="toggleRow(row, $event)"
              />
              <component :is="() => nameCell(row)" />
            </div>
            <NSpace :size="6" align="center" class="inactive-card__meta">
              <component :is="() => kindTag(row)" />
              <component :is="() => appLink(row)" v-if="!appId" />
              <NText v-if="Number(row.child_count)" depth="3" style="font-size: 12px">
                {{ contains(row) }}
              </NText>
            </NSpace>
            <NFlex justify="space-between" align="center">
              <NText depth="3" style="font-size: 12px">{{ formatDate(row.updated_at) }}</NText>
              <component :is="() => deleteButton(row)" />
            </NFlex>
          </div>
        </div>
      </template>

      <NDataTable
        v-else
        v-model:checked-row-keys="checkedKeys"
        :columns="columns"
        :data="pagedRows"
        :loading="loading"
        :row-key="rowKey"
        size="small"
        :bordered="false"
      >
        <template #empty>
          <NEmpty description="No inactive elements" />
        </template>
      </NDataTable>

      <NFlex justify="end">
        <NPagination
          v-model:page="page"
          v-model:page-size="pageSize"
          :item-count="filtered.length"
          :page-sizes="[20, 50, 100]"
          :show-size-picker="!isMobile"
        />
      </NFlex>
    </NSpace>

    <SecretDetailDrawer
      :show="detailShown('secret')"
      :secret-id="detailId('secret')"
      @update:show="showDetail = $event"
    />
    <ItemDetailDrawer
      :show="detailShown('item')"
      :item-id="detailId('item')"
      @update:show="showDetail = $event"
    />
    <ConfigMapDetailDrawer
      :show="detailShown('configmap')"
      :config-map-id="detailId('configmap')"
      @update:show="showDetail = $event"
    />
    <ConfigItemDetailDrawer
      :show="detailShown('config_item')"
      :item-id="detailId('config_item')"
      @update:show="showDetail = $event"
    />

    <InactiveDeleteModal
      v-model:show="showDelete"
      :elements="toDelete"
      @deleted="onDeleted"
      @stale="fetchRows"
    />
  </NCard>
</template>

<style scoped>
.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.filters__select {
  width: 200px;
}

.filters__search {
  width: 220px;
}

.filters__actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

/* Cells are rendered by the table, outside this component's scope. */
:deep(.name-cell) {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

/* Keys can be long: wrap them instead of pushing the table out. */
:deep(.name-cell__name) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: normal;
  word-break: break-all;
  text-align: left;
}

:deep(.name-cell__sub) {
  font-size: 12px;
}

.inactive-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.inactive-card {
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.2));
  border-radius: 8px;
  padding: 10px 12px;
}

.inactive-card__head {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.inactive-card__meta {
  margin: 6px 0;
}

@media (max-width: 768px) {
  .filters__select,
  .filters__search {
    width: 100%;
  }

  .filters__actions {
    width: 100%;
    margin-left: 0;
    justify-content: space-between;
  }
}
</style>
