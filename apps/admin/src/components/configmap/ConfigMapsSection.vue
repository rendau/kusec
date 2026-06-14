<script setup lang="ts">
import { computed, h, provide, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NEllipsis,
  NEmpty,
  NIcon,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  ChevronDown,
  ChevronUp,
  Eye,
  EyeOff,
  InfoCircle,
  Pencil,
  Plus,
  Trash,
} from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { deleteConfigMap, listConfigMaps } from '@/api/configmap'
import type { ConfigMapMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useClipboard } from '@/composables/useClipboard'
import { configItemsRevealCommandKey } from '@/constants/injection'
import type { RevealCommand } from '@/constants/injection'
import {
  configItemsKey,
  createConfigItemsStore,
} from '@/composables/useConfigItems'

import ConfigMapDetailDrawer from '@/components/configmap/ConfigMapDetailDrawer.vue'
import ConfigMapFormModal from '@/components/configmap/ConfigMapFormModal.vue'
import ConfigMapItemsPanel from '@/components/configmap/ConfigMapItemsPanel.vue'

const props = defineProps<{
  /** Application whose config maps are shown. */
  appId: string
}>()

const message = useMessage()
const { copy } = useClipboard()
const { isMobile } = useBreakpoint()

// Auto-expand every config map on load when there are at most this many.
const AUTO_EXPAND_MAX = 5

const rows = ref<ConfigMapMain[]>([])
const loading = ref(false)
const expandedKeys = ref<string[]>([])

function isExpanded(id: string): boolean {
  return expandedKeys.value.includes(id)
}

// Broadcast a one-shot reveal/hide command (separate from the secrets section).
const showAll = ref(false)
const revealCommand = ref<RevealCommand>({ action: 'hide', seq: 0 })
provide(configItemsRevealCommandKey, revealCommand)

// Shared items cache for all panels: loads several config maps' items in a
// single request instead of one request per expanded panel.
const itemsStore = createConfigItemsStore()
provide(configItemsKey, itemsStore)

const allExpanded = computed(
  () => rows.value.length > 0 && expandedKeys.value.length === rows.value.length,
)

function broadcastReveal(show: boolean): void {
  showAll.value = show
  revealCommand.value = {
    action: show ? 'show' : 'hide',
    seq: revealCommand.value.seq + 1,
  }
}

function toggleExpandAll(): void {
  if (allExpanded.value) {
    expandedKeys.value = []
    broadcastReveal(false)
  } else {
    const ids = rows.value.map((r) => r.id)
    void itemsStore.prefetch(ids)
    expandedKeys.value = ids
  }
}

function toggleRevealAll(): void {
  broadcastReveal(!showAll.value)
}

const editing = ref<ConfigMapMain | null>(null)
const showForm = ref(false)

const detailId = ref<string | null>(null)
const showDetail = ref(false)

function openDetail(row: ConfigMapMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function toggleExpand(row: ConfigMapMain): void {
  const set = new Set(expandedKeys.value)
  if (set.has(row.id)) set.delete(row.id)
  else set.add(row.id)
  expandedKeys.value = [...set]
}

async function fetchConfigMaps(): Promise<void> {
  if (!props.appId) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    const rep = await listConfigMaps({ app_id: props.appId })
    rows.value = rep.results ?? []
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load config maps'))
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: ConfigMapMain): void {
  editing.value = row
  showForm.value = true
}

async function removeConfigMap(row: ConfigMapMain): Promise<void> {
  try {
    await deleteConfigMap(row.id)
    message.success('Config map deleted')
    await fetchConfigMaps()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to delete config map'))
  }
}

function iconButton(
  icon: typeof Eye,
  tooltip: string,
  onClick: () => void,
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
            'aria-label': tooltip,
            onClick,
          },
          { icon: () => h(NIcon, { component: icon }) },
        ),
      default: () => tooltip,
    },
  )
}

const columns = computed<DataTableColumns<ConfigMapMain>>(() => [
  {
    type: 'expand',
    renderExpand: (row) => h(ConfigMapItemsPanel, { configmapId: row.id }),
  },
  {
    title: 'Slug',
    key: 'slug_name',
    render: (row) =>
      h(
        NButton,
        {
          text: true,
          type: 'primary',
          block: true,
          style:
            'justify-content: flex-start; padding: 10px 0;' +
            'font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;',
          onClick: () => toggleExpand(row),
        },
        { default: () => row.slug_name },
      ),
  },
  {
    title: 'K8s configmap',
    key: 'kube_configmap_name',
    render: (row) =>
      row.kube_configmap_name
        ? h(
            NTooltip,
            {},
            {
              trigger: () =>
                h(
                  NText,
                  {
                    code: true,
                    style: 'cursor: pointer',
                    onClick: () =>
                      copy(row.kube_configmap_name, 'K8s configmap name copied'),
                  },
                  { default: () => row.kube_configmap_name },
                ),
              default: () => 'Copy',
            },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
  {
    title: 'Description',
    key: 'description',
    render: (row) =>
      row.description
        ? h(
            NEllipsis,
            { style: 'max-width: 320px' },
            { default: () => row.description },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
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
        h(
          NPopconfirm,
          { onPositiveClick: () => removeConfigMap(row) },
          {
            trigger: () =>
              h(
                NButton,
                {
                  quaternary: true,
                  circle: true,
                  size: 'small',
                  type: 'error',
                  'aria-label': 'Delete config map',
                },
                { icon: () => h(NIcon, { component: Trash }) },
              ),
            default: () =>
              `Delete "${row.slug_name}"? This also deletes its items.`,
          },
        ),
      ]),
  },
])

watch(
  () => props.appId,
  async () => {
    expandedKeys.value = []
    itemsStore.reset()
    broadcastReveal(false)
    await fetchConfigMaps()
    // Few config maps → expand them all by default for a quicker overview.
    if (rows.value.length && rows.value.length <= AUTO_EXPAND_MAX) {
      const ids = rows.value.map((r) => r.id)
      void itemsStore.prefetch(ids)
      expandedKeys.value = ids
    }
  },
  { immediate: true },
)

defineExpose({ refresh: fetchConfigMaps, count: computed(() => rows.value.length) })
</script>

<template>
  <NCard title="Config maps" class="stack-header">
    <template #header-extra>
      <NSpace :size="8" align="center">
        <NButton v-if="rows.length" size="small" tertiary @click="toggleExpandAll">
          <template #icon>
            <NIcon :component="allExpanded ? ChevronUp : ChevronDown" />
          </template>
          {{ allExpanded ? 'Collapse all' : 'Expand all' }}
        </NButton>
        <NButton
          v-if="rows.length"
          size="small"
          tertiary
          :type="showAll ? 'warning' : 'default'"
          @click="toggleRevealAll"
        >
          <template #icon>
            <NIcon :component="showAll ? EyeOff : Eye" />
          </template>
          {{ showAll ? 'Hide all' : 'Show all' }}
        </NButton>
        <NButton type="primary" @click="openCreate">
          <template #icon>
            <NIcon :component="Plus" />
          </template>
          New config map
        </NButton>
      </NSpace>
    </template>

    <!-- Mobile: stacked cards instead of a horizontally scrolling table. -->
    <div v-if="isMobile" class="cm-cards">
      <NEmpty
        v-if="!rows.length && !loading"
        description="No config maps yet"
        style="padding: 24px 0"
      />
      <NCard
        v-for="row in rows"
        :key="row.id"
        size="small"
        class="cm-card stack-header"
        :segmented="{ content: true }"
      >
        <template #header>
          <NButton
            text
            type="primary"
            class="cm-card__title"
            @click="toggleExpand(row)"
          >
            {{ row.slug_name }}
          </NButton>
        </template>
        <template #header-extra>
          <NTag :type="row.active ? 'success' : 'default'" size="small">
            {{ row.active ? 'Active' : 'Inactive' }}
          </NTag>
        </template>

        <NSpace vertical :size="6">
          <div v-if="row.kube_configmap_name" class="cm-card__field">
            <NText depth="3" class="cm-card__label">K8s configmap</NText>
            <NText
              code
              class="cm-card__mono"
              @click="copy(row.kube_configmap_name, 'K8s configmap name copied')"
            >
              {{ row.kube_configmap_name }}
            </NText>
          </div>
          <NText v-if="row.description" depth="3" style="font-size: 13px">
            {{ row.description }}
          </NText>
        </NSpace>

        <template #action>
          <NSpace justify="space-between" align="center">
            <NButton size="small" tertiary @click="toggleExpand(row)">
              <template #icon>
                <NIcon :component="isExpanded(row.id) ? ChevronUp : ChevronDown" />
              </template>
              {{ isExpanded(row.id) ? 'Hide items' : 'Items' }}
            </NButton>
            <NSpace :size="4" :wrap-item="false">
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
                aria-label="Edit config map"
                @click="openEdit(row)"
              >
                <template #icon>
                  <NIcon :component="Pencil" />
                </template>
              </NButton>
              <NPopconfirm @positive-click="removeConfigMap(row)">
                <template #trigger>
                  <NButton
                    quaternary
                    circle
                    size="small"
                    type="error"
                    aria-label="Delete config map"
                  >
                    <template #icon>
                      <NIcon :component="Trash" />
                    </template>
                  </NButton>
                </template>
                Delete "{{ row.slug_name }}"? This also deletes its items.
              </NPopconfirm>
            </NSpace>
          </NSpace>
          <ConfigMapItemsPanel v-if="isExpanded(row.id)" :configmap-id="row.id" />
        </template>
      </NCard>
    </div>

    <NDataTable
      v-else
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(row: ConfigMapMain) => row.id"
      :pagination="false"
      :theme-overrides="{ tdColorHover: 'transparent' }"
      :expanded-row-keys="expandedKeys"
      @update:expanded-row-keys="
        (keys: (string | number)[]) => (expandedKeys = keys.map(String))
      "
    />
  </NCard>

  <ConfigMapDetailDrawer v-model:show="showDetail" :config-map-id="detailId" />
  <ConfigMapFormModal
    v-model:show="showForm"
    :config-map="editing"
    :default-app-id="appId"
    lock-app
    @saved="fetchConfigMaps"
  />
</template>

<style scoped>
.cm-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cm-card__title {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-weight: 600;
  max-width: 100%;
}

/* Let a long slug wrap instead of overflowing the card (no horizontal scroll). */
.cm-card__title :deep(.n-button__content) {
  white-space: normal;
  word-break: break-word;
  text-align: left;
}

.cm-card__field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cm-card__label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.cm-card__mono {
  cursor: pointer;
  word-break: break-all;
}
</style>
