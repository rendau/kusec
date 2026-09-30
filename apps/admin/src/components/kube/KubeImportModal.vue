<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import {
  NButton,
  NDataTable,
  NEmpty,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NRadio,
  NRadioButton,
  NRadioGroup,
  NResult,
  NScrollbar,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { Refresh, Search } from '@vicons/tabler'
import { storeToRefs } from 'pinia'

import { ApiError, apiErrorMessage } from '@/api/http'
import {
  importClusterConfigMap,
  importClusterSecret,
  listClusterConfigMaps,
  listClusterSecrets,
} from '@/api/kube'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useAppsStore } from '@/stores/apps'

const props = defineProps<{
  show: boolean
  /** Preselect this application as the import target (e.g. from a workspace). */
  defaultAppId?: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  /** Emitted after a successful import so callers can refresh their data. */
  imported: []
}>()

const message = useMessage()
const { isMobile } = useBreakpoint()

const appsStore = useAppsStore()
const { apps } = storeToRefs(appsStore)

/** What is being imported: cluster secrets or config maps. */
type ImportKind = 'secret' | 'configmap'

/** One cluster object offered for import; config maps have no `type`. */
interface ClusterRow {
  namespace: string
  name: string
  type: string
  keys: string[]
  managed: boolean
}

/** Per-kind UI texts (the flow itself is identical for both kinds). */
const KIND_TEXT: Record<
  ImportKind,
  { column: string; one: string; many: string; landing: string }
> = {
  secret: {
    column: 'Secret',
    one: 'secret',
    many: 'secrets',
    landing: 'Landing secret name',
  },
  configmap: {
    column: 'Config map',
    one: 'config map',
    many: 'config maps',
    landing: 'Landing config map name',
  },
}

const loading = ref(false)
const importing = ref(false)
const inCluster = ref(true)
const kind = ref<ImportKind>('secret')
const rows = ref<ClusterRow[]>([])

const text = computed(() => KIND_TEXT[kind.value])

// Один объект за импорт: выбранный ключ строки и имя посадочной записи.
const selectedKey = ref<string | null>(null)
const secretSlug = ref('')

const targetAppId = ref<string | null>(null)
const namespaceFilter = ref<string>('')
const search = ref('')

/** Currently selected cluster object (single selection). */
const selectedSecret = computed<ClusterRow | null>(
  () => rows.value.find((s) => rowKey(s) === selectedKey.value) ?? null,
)

// При выборе объекта подставляем его имя как имя посадочной записи —
// пользователь может отредактировать перед импортом.
watch(selectedSecret, (secret) => {
  secretSlug.value = secret?.name ?? ''
})

/** Target application options (`name · namespace`). */
const appOptions = computed<SelectOption[]>(() =>
  apps.value.map((app) => ({
    label: app.namespace ? `${app.name} · ${app.namespace}` : app.name,
    value: app.id,
  })),
)

/** Stable row key — a cluster object is unique by namespace + name. */
function rowKey(row: ClusterRow): string {
  return `${row.namespace}/${row.name}`
}

/** Coerce an int64-as-string counter into a number for display. */
function toCount(value: number | string | undefined): number {
  const parsed = Number(value ?? 0)
  return Number.isFinite(parsed) ? parsed : 0
}

const namespaceOptions = computed<SelectOption[]>(() => {
  const names = [...new Set(rows.value.map((s) => s.namespace))].sort()
  return [
    { label: 'All namespaces', value: '' },
    ...names.map((name) => ({ label: name, value: name })),
  ]
})

const filteredSecrets = computed<ClusterRow[]>(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter((s) => {
    if (namespaceFilter.value && s.namespace !== namespaceFilter.value) return false
    if (!q) return true
    return (
      s.name.toLowerCase().includes(q) ||
      s.namespace.toLowerCase().includes(q) ||
      s.keys.some((k) => k.toLowerCase().includes(q))
    )
  })
})

const columns = computed<DataTableColumns<ClusterRow>>(() => [
  { type: 'selection', multiple: false },
  {
    title: 'Namespace',
    key: 'namespace',
    width: 140,
    ellipsis: { tooltip: true },
    render: (row) =>
      h(NTag, { size: 'small', type: 'info', bordered: false }, () => row.namespace),
  },
  {
    title: text.value.column,
    key: 'name',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { code: true }, () => row.name),
  },
  // Тип есть только у секретов.
  ...(kind.value === 'secret'
    ? ([
        {
          title: 'Type',
          key: 'type',
          width: 140,
          ellipsis: { tooltip: true },
          render: (row) =>
            row.type
              ? h(NText, { code: true }, () => row.type)
              : h(NText, { depth: 3 }, () => 'Opaque'),
        },
      ] as DataTableColumns<ClusterRow>)
    : []),
  {
    title: 'Keys',
    key: 'keys',
    minWidth: 120,
    ellipsis: { tooltip: { contentStyle: 'max-width: 320px' } },
    render: (row) =>
      row.keys.length
        ? h(NText, { depth: 2 }, () => `${row.keys.length}: ${row.keys.join(', ')}`)
        : h(NText, { depth: 3 }, () => 'empty'),
  },
  {
    title: '',
    key: 'managed',
    width: 96,
    render: (row) =>
      row.managed
        ? h(
            NTag,
            {
              size: 'small',
              type: 'success',
              bordered: false,
              title: 'Already managed by kusec',
            },
            () => 'managed',
          )
        : null,
  },
])

const canImport = computed(
  () =>
    inCluster.value &&
    !!targetAppId.value &&
    !!selectedKey.value &&
    secretSlug.value.trim().length > 0,
)

async function load(): Promise<void> {
  // Запоминаем вид на момент запроса: ответ устаревшего запроса (вид
  // переключили, пока он шёл) отбрасываем.
  const requested = kind.value
  loading.value = true
  try {
    let loaded: ClusterRow[]
    let loadedInCluster: boolean
    if (requested === 'secret') {
      const rep = await listClusterSecrets()
      loadedInCluster = rep.in_cluster
      loaded = rep.secrets ?? []
    } else {
      const rep = await listClusterConfigMaps()
      loadedInCluster = rep.in_cluster
      loaded = (rep.configmaps ?? []).map((cm) => ({ ...cm, type: '' }))
    }
    if (requested !== kind.value) return
    inCluster.value = loadedInCluster
    rows.value = loaded
  } catch (error) {
    if (requested !== kind.value) return
    if (error instanceof ApiError && error.code === 'no_permission') {
      inCluster.value = true
      rows.value = []
    }
    message.error(apiErrorMessage(error, `Failed to load cluster ${text.value.many}`))
  } finally {
    if (requested === kind.value) loading.value = false
  }
}

/** Reset the selection and filters (on open and on kind switch). */
function resetSelection(): void {
  selectedKey.value = null
  secretSlug.value = ''
  namespaceFilter.value = ''
  search.value = ''
}

// Переключение вида: другой список, выбор сбрасывается.
watch(kind, () => {
  if (!props.show) return
  resetSelection()
  rows.value = []
  void load()
})

watch(
  () => props.show,
  (show) => {
    if (!show) return
    resetSelection()
    void appsStore.ensureLoaded()
    // Preselect the requested target, else the only app, else nothing.
    targetAppId.value =
      props.defaultAppId ??
      (apps.value.length === 1 ? (apps.value[0]?.id ?? null) : null)
    void load()
  },
)

function close(): void {
  emit('update:show', false)
}

async function submit(): Promise<void> {
  const secret = selectedSecret.value
  const slug = secretSlug.value.trim()
  if (!targetAppId.value || !secret || !slug) return

  importing.value = true
  try {
    const ref = { namespace: secret.namespace, name: secret.name }
    let created: boolean
    let landedSlug: string
    let createdItems: number | string
    let updatedItems: number | string
    if (kind.value === 'secret') {
      const rep = await importClusterSecret(targetAppId.value, ref, slug)
      created = rep.secret_created
      landedSlug = rep.secret_slug
      createdItems = rep.created_items
      updatedItems = rep.updated_items
    } else {
      const rep = await importClusterConfigMap(targetAppId.value, ref, slug)
      created = rep.configmap_created
      landedSlug = rep.configmap_slug
      createdItems = rep.created_items
      updatedItems = rep.updated_items
    }
    const verb = created ? 'Imported' : 'Topped up'
    const updated = toCount(updatedItems)
    const parts = [`+${toCount(createdItems)} items`]
    if (updated > 0) parts.push(`${updated} overridden`)
    message.success(`${verb} "${landedSlug}" · ${parts.join(' · ')}`)
    emit('imported')
    close()
  } catch (error) {
    const hint =
      error instanceof ApiError && error.code === 'not_in_cluster'
        ? 'The service is not running inside a Kubernetes cluster'
        : undefined
    message.error(hint ?? apiErrorMessage(error, 'Import failed'))
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Import from cluster"
    class="kube-import-modal"
    style="width: 820px; max-width: calc(100vw - 32px)"
    :mask-closable="!importing"
    @update:show="emit('update:show', $event)"
  >
    <NSpin :show="loading">
      <NResult
        v-if="!loading && !inCluster"
        status="info"
        title="Not running in a cluster"
        description="Cluster import is available only when the service runs inside Kubernetes."
      />

      <template v-else>
        <NSpace vertical :size="12">
          <NRadioGroup
            v-model:value="kind"
            size="small"
            :disabled="importing"
            aria-label="What to import"
          >
            <NRadioButton value="secret">Secrets</NRadioButton>
            <NRadioButton value="configmap">Config maps</NRadioButton>
          </NRadioGroup>

          <NText depth="3" style="font-size: 12px">
            Pick the target application and one cluster {{ text.one }} to import, then
            name the landing kusec {{ text.one }}. If a {{ text.one }} with that name
            already exists, missing keys are added and matching keys are overridden. The
            cluster source is left unchanged.
          </NText>

          <NFormItem label="Target application" :show-feedback="false">
            <NSelect
              v-model:value="targetAppId"
              :options="appOptions"
              filterable
              placeholder="Choose an application"
            />
          </NFormItem>

          <NFormItem :label="text.landing" :show-feedback="false">
            <NInput
              v-model:value="secretSlug"
              :placeholder="`kusec ${text.one} slug (required)`"
              :disabled="!selectedKey"
            />
          </NFormItem>

          <div class="kube-import-modal__filters">
            <NSelect
              v-model:value="namespaceFilter"
              :options="namespaceOptions"
              size="small"
              class="kube-import-modal__ns"
            />
            <NInput
              v-model:value="search"
              size="small"
              :placeholder="`Search ${text.one}, namespace or key`"
              clearable
            >
              <template #prefix>
                <NIcon :component="Search" />
              </template>
            </NInput>
            <NButton size="small" tertiary :loading="loading" @click="load">
              <template #icon>
                <NIcon :component="Refresh" />
              </template>
            </NButton>
          </div>

          <!-- Mobile: stacked cards (no horizontal scroll). -->
          <NScrollbar v-if="isMobile" class="kube-import-modal__cards">
            <NEmpty
              v-if="!filteredSecrets.length"
              size="small"
              :description="`No cluster ${text.many} found`"
              style="padding: 24px 0"
            />
            <NRadioGroup v-else v-model:value="selectedKey" class="kube-cards">
              <label
                v-for="row in filteredSecrets"
                :key="rowKey(row)"
                class="kube-card"
              >
                <NRadio :value="rowKey(row)" class="kube-card__check" />
                <div class="kube-card__body">
                  <div class="kube-card__row">
                    <NText code class="kube-card__name">{{ row.name }}</NText>
                    <NTag
                      v-if="row.managed"
                      size="tiny"
                      type="success"
                      :bordered="false"
                    >
                      managed
                    </NTag>
                  </div>
                  <NSpace :size="6" align="center" style="margin-top: 4px">
                    <NTag size="tiny" type="info" :bordered="false">
                      {{ row.namespace }}
                    </NTag>
                    <NText v-if="kind === 'secret'" depth="3" style="font-size: 12px">
                      {{ row.type || 'Opaque' }}
                    </NText>
                  </NSpace>
                  <NText depth="3" class="kube-card__keys">
                    {{
                      row.keys.length
                        ? `${row.keys.length} keys: ${row.keys.join(', ')}`
                        : 'no keys'
                    }}
                  </NText>
                </div>
              </label>
            </NRadioGroup>
          </NScrollbar>

          <!-- Desktop: dense table with row selection. -->
          <NDataTable
            v-else
            :columns="columns"
            :data="filteredSecrets"
            :row-key="rowKey"
            :checked-row-keys="selectedKey ? [selectedKey] : []"
            max-height="50vh"
            size="small"
            :pagination="false"
            @update:checked-row-keys="
              (keys) => (selectedKey = keys.length ? String(keys[keys.length - 1]) : null)
            "
          >
            <template #empty>
              <NEmpty size="small" :description="`No cluster ${text.many} found`" />
            </template>
          </NDataTable>
        </NSpace>
      </template>
    </NSpin>

    <template #footer>
      <NSpace justify="space-between" align="center">
        <NText depth="3" style="font-size: 13px">
          {{ selectedSecret ? `${selectedSecret.namespace}/${selectedSecret.name}` : 'none selected' }}
        </NText>
        <NSpace>
          <NButton :disabled="importing" @click="close">Cancel</NButton>
          <NButton
            type="primary"
            :loading="importing"
            :disabled="!canImport"
            @click="submit"
          >
            Import
          </NButton>
        </NSpace>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.kube-import-modal__filters {
  display: flex;
  align-items: center;
  gap: 8px;
}

.kube-import-modal__ns {
  width: 200px;
  flex: 0 0 auto;
}

.kube-import-modal__cards {
  max-height: 50vh;
}

.kube-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.kube-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid rgba(128, 128, 128, 0.22);
  border-radius: 8px;
  cursor: pointer;
}

.kube-card__check {
  margin-top: 2px;
}

.kube-card__body {
  min-width: 0;
  flex: 1;
}

.kube-card__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.kube-card__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kube-card__keys {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  word-break: break-word;
}

@media (max-width: 600px) {
  .kube-import-modal__filters {
    flex-wrap: wrap;
  }

  .kube-import-modal__ns {
    width: 100%;
  }
}
</style>
