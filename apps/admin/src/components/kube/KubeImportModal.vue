<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import {
  NButton,
  NCheckbox,
  NCheckboxGroup,
  NDataTable,
  NEmpty,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NResult,
  NScrollbar,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
  useNotification,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { Refresh, Search } from '@vicons/tabler'
import { storeToRefs } from 'pinia'

import { ApiError, apiErrorMessage } from '@/api/http'
import { importClusterSecrets, listClusterSecrets } from '@/api/kube'
import type { KubeClusterSecretSt } from '@/api/types'
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
const notification = useNotification()
const { isMobile } = useBreakpoint()

const appsStore = useAppsStore()
const { apps } = storeToRefs(appsStore)

const loading = ref(false)
const importing = ref(false)
const inCluster = ref(true)
const secrets = ref<KubeClusterSecretSt[]>([])
const checkedKeys = ref<string[]>([])

const targetAppId = ref<string | null>(null)
const namespaceFilter = ref<string>('')
const search = ref('')

/** Target application options (`name · namespace`). */
const appOptions = computed<SelectOption[]>(() =>
  apps.value.map((app) => ({
    label: app.namespace ? `${app.name} · ${app.namespace}` : app.name,
    value: app.id,
  })),
)

/** Stable row key — a cluster secret is unique by namespace + name. */
function rowKey(row: KubeClusterSecretSt): string {
  return `${row.namespace}/${row.name}`
}

/** Coerce an int64-as-string counter into a number for display. */
function toCount(value: number | string | undefined): number {
  const parsed = Number(value ?? 0)
  return Number.isFinite(parsed) ? parsed : 0
}

const namespaceOptions = computed<SelectOption[]>(() => {
  const names = [...new Set(secrets.value.map((s) => s.namespace))].sort()
  return [
    { label: 'All namespaces', value: '' },
    ...names.map((name) => ({ label: name, value: name })),
  ]
})

const filteredSecrets = computed<KubeClusterSecretSt[]>(() => {
  const q = search.value.trim().toLowerCase()
  return secrets.value.filter((s) => {
    if (namespaceFilter.value && s.namespace !== namespaceFilter.value) return false
    if (!q) return true
    return (
      s.name.toLowerCase().includes(q) ||
      s.namespace.toLowerCase().includes(q) ||
      s.keys.some((k) => k.toLowerCase().includes(q))
    )
  })
})

const columns = computed<DataTableColumns<KubeClusterSecretSt>>(() => [
  { type: 'selection' },
  {
    title: 'Namespace',
    key: 'namespace',
    width: 140,
    ellipsis: { tooltip: true },
    render: (row) =>
      h(NTag, { size: 'small', type: 'info', bordered: false }, () => row.namespace),
  },
  {
    title: 'Secret',
    key: 'name',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { code: true }, () => row.name),
  },
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

const selectedCount = computed(() => checkedKeys.value.length)

async function load(): Promise<void> {
  loading.value = true
  try {
    const rep = await listClusterSecrets()
    inCluster.value = rep.in_cluster
    secrets.value = rep.secrets ?? []
  } catch (error) {
    if (error instanceof ApiError && error.code === 'no_permission') {
      inCluster.value = true
      secrets.value = []
    }
    message.error(apiErrorMessage(error, 'Failed to load cluster secrets'))
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (!show) return
    checkedKeys.value = []
    namespaceFilter.value = ''
    search.value = ''
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
  if (!targetAppId.value) return

  const byKey = new Map(secrets.value.map((s) => [rowKey(s), s]))
  const refs = checkedKeys.value
    .map((key) => byKey.get(key))
    .filter((s): s is KubeClusterSecretSt => s != null)
    .map((s) => ({ namespace: s.namespace, name: s.name }))

  if (!refs.length) return

  importing.value = true
  try {
    const rep = await importClusterSecrets(targetAppId.value, refs)
    const summary = [
      `imported ${rep.imported?.length ?? 0}`,
      `skipped ${rep.skipped?.length ?? 0}`,
      `+${toCount(rep.created_secrets)} secrets`,
      `+${toCount(rep.created_items)} items`,
    ].join(' · ')

    if (rep.errors?.length) {
      notification.warning({
        title: 'Import finished with errors',
        content: `${summary}\n\n${rep.errors.join('\n')}`,
        duration: 0,
      })
    } else {
      notification.success({
        title: 'Cluster secrets imported',
        content: summary,
        duration: 6000,
      })
    }

    if (toCount(rep.created_secrets) > 0) emit('imported')
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
    title="Import secrets from cluster"
    class="kube-import-modal"
    :mask-closable="!importing"
    @update:show="emit('update:show', $event)"
  >
    <NSpin :show="loading">
      <NResult
        v-if="!loading && !inCluster"
        status="info"
        title="Not running in a cluster"
        description="Cluster secret import is available only when the service runs inside Kubernetes."
      />

      <template v-else>
        <NSpace vertical :size="12">
          <NText depth="3" style="font-size: 12px">
            Pick the target application and the cluster secrets to import. Each
            secret becomes a kusec secret (named after its cluster name) with one
            item per key. The cluster source is left unchanged.
          </NText>

          <NFormItem label="Target application" :show-feedback="false">
            <NSelect
              v-model:value="targetAppId"
              :options="appOptions"
              filterable
              placeholder="Choose an application"
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
              placeholder="Search secret, namespace or key"
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
              description="No cluster secrets found"
              style="padding: 24px 0"
            />
            <NCheckboxGroup v-else v-model:value="checkedKeys" class="kube-cards">
              <label
                v-for="row in filteredSecrets"
                :key="rowKey(row)"
                class="kube-card"
              >
                <NCheckbox :value="rowKey(row)" class="kube-card__check" />
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
                    <NText depth="3" style="font-size: 12px">
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
            </NCheckboxGroup>
          </NScrollbar>

          <!-- Desktop: dense table with row selection. -->
          <NDataTable
            v-else
            :columns="columns"
            :data="filteredSecrets"
            :row-key="rowKey"
            :checked-row-keys="checkedKeys"
            max-height="50vh"
            size="small"
            :pagination="false"
            @update:checked-row-keys="(keys) => (checkedKeys = keys.map(String))"
          >
            <template #empty>
              <NEmpty size="small" description="No cluster secrets found" />
            </template>
          </NDataTable>
        </NSpace>
      </template>
    </NSpin>

    <template #footer>
      <NSpace justify="space-between" align="center">
        <NText depth="3" style="font-size: 13px"> {{ selectedCount }} selected </NText>
        <NSpace>
          <NButton :disabled="importing" @click="close">Cancel</NButton>
          <NButton
            type="primary"
            :loading="importing"
            :disabled="!selectedCount || !inCluster || !targetAppId"
            @click="submit"
          >
            Import {{ selectedCount || '' }}
          </NButton>
        </NSpace>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.kube-import-modal {
  width: 820px;
  max-width: calc(100vw - 32px);
}

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
