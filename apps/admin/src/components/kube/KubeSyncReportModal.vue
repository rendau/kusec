<script setup lang="ts">
import { computed, h } from 'vue'
import {
  NAlert,
  NButton,
  NDataTable,
  NEmpty,
  NModal,
  NSpace,
  NTag,
  NText,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

import type { KubeSyncConfigMapsRep, KubeSyncSecretsRep } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import type { KubeSyncReportSt } from '@/composables/useKubeSync'

const props = defineProps<{
  show: boolean
  report: KubeSyncReportSt | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { isMobile } = useBreakpoint()

type SyncKind = 'Secret' | 'Config map'
type SyncAction = 'created' | 'updated' | 'deleted'

interface ChangeRow {
  key: string
  kind: SyncKind
  name: string
  action: SyncAction
}

const ACTION_TAG_TYPE: Record<SyncAction, 'success' | 'info' | 'error'> = {
  created: 'success',
  updated: 'info',
  deleted: 'error',
}

/** Coerce an int64-as-string counter into a number for display. */
function toCount(value: number | string | undefined): number {
  const parsed = Number(value ?? 0)
  return Number.isFinite(parsed) ? parsed : 0
}

function kindRows(
  kind: SyncKind,
  rep: KubeSyncSecretsRep | KubeSyncConfigMapsRep,
): ChangeRow[] {
  const actions: SyncAction[] = ['created', 'updated', 'deleted']
  return actions.flatMap((action) =>
    (rep[action] ?? []).map((name) => ({
      key: `${kind}:${action}:${name}`,
      kind,
      name,
      action,
    })),
  )
}

/** Flat change list: secrets first, then config maps; created → updated → deleted. */
const rows = computed<ChangeRow[]>(() =>
  props.report
    ? [
        ...kindRows('Secret', props.report.secrets),
        ...kindRows('Config map', props.report.configmaps),
      ]
    : [],
)

const errors = computed<{ kind: SyncKind; text: string }[]>(() =>
  props.report
    ? [
        ...(props.report.secrets.errors ?? []).map((text) => ({
          kind: 'Secret' as const,
          text,
        })),
        ...(props.report.configmaps.errors ?? []).map((text) => ({
          kind: 'Config map' as const,
          text,
        })),
      ]
    : [],
)

/** Per-kind counters for the summary strip ("Secrets: created 2 · …"). */
const stats = computed(() => {
  if (!props.report) return []
  const kinds: { label: string; rep: KubeSyncSecretsRep | KubeSyncConfigMapsRep }[] = [
    { label: 'Secrets', rep: props.report.secrets },
    { label: 'Config maps', rep: props.report.configmaps },
  ]
  return kinds.map(({ label, rep }) => ({
    label,
    counters: [
      { action: 'created' as const, count: rep.created?.length ?? 0 },
      { action: 'updated' as const, count: rep.updated?.length ?? 0 },
      { action: 'deleted' as const, count: rep.deleted?.length ?? 0 },
      { action: 'unchanged', count: toCount(rep.unchanged) },
    ],
  }))
})

const finishedAtLabel = computed(() =>
  props.report ? props.report.finishedAt.toLocaleString() : '',
)

const columns: DataTableColumns<ChangeRow> = [
  {
    title: 'Action',
    key: 'action',
    width: 110,
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: ACTION_TAG_TYPE[row.action], bordered: false },
        () => row.action,
      ),
  },
  {
    title: 'Kind',
    key: 'kind',
    width: 120,
    render: (row) => h(NText, { depth: 2 }, () => row.kind),
  },
  {
    title: 'Resource (namespace/name)',
    key: 'name',
    minWidth: 220,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { code: true }, () => row.name),
  },
]

function close(): void {
  emit('update:show', false)
}
</script>

<template>
  <!-- The report stays on screen until explicitly closed: no auto-dismiss,
       no accidental mask/esc close — the user confirms they have read it. -->
  <NModal
    :show="show && !!report"
    preset="card"
    title="Kubernetes sync report"
    class="kube-sync-report"
    style="width: 720px; max-width: calc(100vw - 32px)"
    :mask-closable="false"
    :close-on-esc="false"
    @update:show="emit('update:show', $event)"
  >
    <NSpace v-if="report" vertical :size="14">
      <NText depth="3" style="font-size: 12px">
        Scope: {{ report.scope }} · finished at {{ finishedAtLabel }}
      </NText>

      <NAlert
        v-if="errors.length"
        type="error"
        :title="`Finished with ${errors.length} error${errors.length > 1 ? 's' : ''}`"
      >
        <ul class="kube-sync-report__errors">
          <li v-for="(err, i) in errors" :key="i">
            <NTag size="tiny" :bordered="false">{{ err.kind }}</NTag>
            <span class="kube-sync-report__error-text">{{ err.text }}</span>
          </li>
        </ul>
      </NAlert>
      <NAlert v-else type="success" title="Finished successfully">
        The cluster now matches the active records in kusec.
      </NAlert>

      <!-- Per-kind counters; zero counters are dimmed so real changes stand out. -->
      <div class="kube-sync-report__stats">
        <div v-for="stat in stats" :key="stat.label" class="kube-sync-report__stat">
          <NText depth="2" class="kube-sync-report__stat-label">
            {{ stat.label }}
          </NText>
          <NSpace :size="6">
            <NTag
              v-for="counter in stat.counters"
              :key="counter.action"
              size="small"
              :bordered="false"
              :type="
                counter.count > 0 && counter.action !== 'unchanged'
                  ? ACTION_TAG_TYPE[counter.action as SyncAction]
                  : 'default'
              "
              :class="{ 'kube-sync-report__zero': counter.count === 0 }"
            >
              {{ counter.action }} {{ counter.count }}
            </NTag>
          </NSpace>
        </div>
      </div>

      <NEmpty
        v-if="!rows.length"
        size="small"
        description="No changes — the cluster was already in sync"
        style="padding: 16px 0"
      />

      <!-- Mobile: stacked rows instead of a table (no horizontal scroll). -->
      <div v-else-if="isMobile" class="kube-sync-report__cards">
        <div v-for="row in rows" :key="row.key" class="kube-sync-report__card">
          <NTag size="small" :type="ACTION_TAG_TYPE[row.action]" :bordered="false">
            {{ row.action }}
          </NTag>
          <div class="kube-sync-report__card-body">
            <NText code class="kube-sync-report__card-name">{{ row.name }}</NText>
            <NText depth="3" style="font-size: 12px">{{ row.kind }}</NText>
          </div>
        </div>
      </div>

      <NDataTable
        v-else
        :columns="columns"
        :data="rows"
        :row-key="(row: ChangeRow) => row.key"
        max-height="45vh"
        size="small"
        :pagination="false"
      />
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton type="primary" @click="close">Close</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.kube-sync-report__errors {
  margin: 0;
  padding-left: 18px;
}

.kube-sync-report__errors li {
  margin-bottom: 4px;
}

.kube-sync-report__error-text {
  margin-left: 6px;
  word-break: break-word;
}

.kube-sync-report__stats {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kube-sync-report__stat {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.kube-sync-report__stat-label {
  min-width: 96px;
  font-size: 13px;
}

.kube-sync-report__zero {
  opacity: 0.55;
}

.kube-sync-report__cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 45vh;
  overflow-y: auto;
}

.kube-sync-report__card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid rgba(128, 128, 128, 0.22);
  border-radius: 8px;
}

.kube-sync-report__card-body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.kube-sync-report__card-name {
  overflow-wrap: anywhere;
}
</style>
