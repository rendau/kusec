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
  useThemeVars,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  ChevronDown,
  ChevronUp,
  InfoCircle,
  Pencil,
  Plus,
  Trash,
} from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { deleteSecret, listSecrets } from '@/api/secret'
import type { SecretMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useClipboard } from '@/composables/useClipboard'
import { createSecretItemsStore, secretItemsKey } from '@/composables/useSecretItems'
import { useChunkedExpand } from '@/composables/useChunkedExpand'

import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'
import SecretFormModal from '@/components/secret/SecretFormModal.vue'
import SecretItemsPanel from '@/components/secret/SecretItemsPanel.vue'

const props = defineProps<{
  /** Application whose secrets are shown. */
  appId: string
}>()

const message = useMessage()
const themeVars = useThemeVars()
const { copy } = useClipboard()
const { isMobile } = useBreakpoint()

// Auto-expand every secret on load when there are at most this many.
const AUTO_EXPAND_MAX = 5

const rows = ref<SecretMain[]>([])
const loading = ref(false)
const expandedKeys = ref<string[]>([])

// "Expand all" reveals panels a few per frame instead of all in one task.
const { expandAll: expandAllChunked, cancel: cancelExpand } =
  useChunkedExpand(expandedKeys)

function isExpanded(id: string): boolean {
  return expandedKeys.value.includes(id)
}

// Shared items cache for all panels: lets us load several secrets' items in a
// single request instead of one request per expanded panel.
const itemsStore = createSecretItemsStore()
provide(secretItemsKey, itemsStore)

const allExpanded = computed(
  () => rows.value.length > 0 && expandedKeys.value.length === rows.value.length,
)

function toggleExpandAll(): void {
  if (allExpanded.value) {
    cancelExpand()
    expandedKeys.value = []
  } else {
    // Items are already cached by the app-wide prefetch — this is a no-op
    // safety net (e.g. after a failed load).
    void itemsStore.prefetchApp(props.appId, rows.value.map((r) => r.id))
    void expandAllChunked(rows.value.map((r) => r.id))
  }
}

const editing = ref<SecretMain | null>(null)
const showForm = ref(false)

const detailId = ref<string | null>(null)
const showDetail = ref(false)

function openDetail(row: SecretMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function toggleExpand(row: SecretMain): void {
  const set = new Set(expandedKeys.value)
  if (set.has(row.id)) set.delete(row.id)
  else set.add(row.id)
  expandedKeys.value = [...set]
}

async function fetchSecrets(): Promise<void> {
  if (!props.appId) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    // Scoped by app_id → backend returns all secrets, no pagination needed.
    const rep = await listSecrets({ app_id: props.appId })
    rows.value = rep.results ?? []
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load secrets'))
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: SecretMain): void {
  editing.value = row
  showForm.value = true
}

async function removeSecret(row: SecretMain): Promise<void> {
  try {
    await deleteSecret(row.id)
    message.success('Secret deleted')
    await fetchSecrets()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to delete secret'))
  }
}

function iconButton(
  icon: typeof Pencil,
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

// Every secret row is tinted (`row--parent`) so it stands apart from its items
// once expanded; inactive ones are also dimmed.
function rowClassName(row: SecretMain): string {
  return row.active ? 'row--parent' : 'row--parent row--inactive'
}

const columns = computed<DataTableColumns<SecretMain>>(() => [
  {
    type: 'expand',
    renderExpand: (row) =>
      h(SecretItemsPanel, { secretId: row.id, parentInactive: !row.active }),
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
          // Click the name to expand/collapse the row (View has its own button).
          style:
            'justify-content: flex-start; padding: 10px 0;' +
            'font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;',
          onClick: () => toggleExpand(row),
        },
        { default: () => row.slug_name },
      ),
  },
  {
    title: 'K8s secret',
    key: 'kube_secret_name',
    render: (row) =>
      row.kube_secret_name
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
                      copy(row.kube_secret_name, 'K8s secret name copied'),
                  },
                  { default: () => row.kube_secret_name },
                ),
              default: () => 'Copy',
            },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
  {
    title: 'K8s type',
    key: 'kube_type',
    render: (row) =>
      row.kube_type
        ? h(NText, { code: true }, { default: () => row.kube_type })
        : h(NText, { depth: 3 }, { default: () => 'Opaque' }),
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
    className: 'cell--full',
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
    className: 'cell--full',
    render: (row) =>
      h(NSpace, { size: 4, wrapItem: false }, () => [
        iconButton(InfoCircle, 'Details', () => openDetail(row)),
        iconButton(Pencil, 'Edit', () => openEdit(row)),
        h(
          NPopconfirm,
          { onPositiveClick: () => removeSecret(row) },
          {
            trigger: () =>
              h(
                NButton,
                {
                  quaternary: true,
                  circle: true,
                  size: 'small',
                  type: 'error',
                  'aria-label': 'Delete secret',
                },
                { icon: () => h(NIcon, { component: Trash }) },
              ),
            default: () => `Delete "${row.slug_name}"? This also deletes its items.`,
          },
        ),
      ]),
  },
])

watch(
  () => props.appId,
  async () => {
    cancelExpand()
    expandedKeys.value = []
    itemsStore.reset()
    await fetchSecrets()
    if (rows.value.length) {
      // Load every item of the app in one request up front, so expanding
      // (individually or via "Expand all") never hits the network again.
      void itemsStore.prefetchApp(props.appId, rows.value.map((r) => r.id))
      // Few secrets → expand them all by default for quicker overview.
      if (rows.value.length <= AUTO_EXPAND_MAX) {
        expandedKeys.value = rows.value.map((r) => r.id)
      }
    }
  },
  { immediate: true },
)

defineExpose({ refresh: fetchSecrets, count: computed(() => rows.value.length) })
</script>

<template>
  <div class="secrets-section">
    <NSpace :size="8" align="center" justify="end" class="secrets-section__actions">
      <NButton v-if="rows.length" size="small" tertiary @click="toggleExpandAll">
        <template #icon>
          <NIcon :component="allExpanded ? ChevronUp : ChevronDown" />
        </template>
        {{ allExpanded ? 'Collapse all' : 'Expand all' }}
      </NButton>
      <NButton type="primary" @click="openCreate">
        <template #icon>
          <NIcon :component="Plus" />
        </template>
        New secret
      </NButton>
    </NSpace>

    <!-- Mobile: stacked cards instead of a horizontally scrolling table. -->
    <div v-if="isMobile" class="secret-cards">
      <NEmpty
        v-if="!rows.length && !loading"
        description="No secrets yet"
        style="padding: 24px 0"
      />
      <NCard
        v-for="row in rows"
        :key="row.id"
        size="small"
        class="secret-card stack-header"
        :class="{ 'secret-card--inactive': !row.active }"
        :segmented="{ content: true }"
      >
        <template #header>
          <NButton
            text
            type="primary"
            class="secret-card__title"
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
          <div v-if="row.kube_secret_name" class="secret-card__field">
            <NText depth="3" class="secret-card__label">K8s secret</NText>
            <NText
              code
              class="secret-card__mono"
              @click="copy(row.kube_secret_name, 'K8s secret name copied')"
            >
              {{ row.kube_secret_name }}
            </NText>
          </div>
          <div class="secret-card__field">
            <NText depth="3" class="secret-card__label">K8s type</NText>
            <NText v-if="row.kube_type" code>{{ row.kube_type }}</NText>
            <NText v-else :depth="3">Opaque</NText>
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
                aria-label="Edit secret"
                @click="openEdit(row)"
              >
                <template #icon>
                  <NIcon :component="Pencil" />
                </template>
              </NButton>
              <NPopconfirm @positive-click="removeSecret(row)">
                <template #trigger>
                  <NButton
                    quaternary
                    circle
                    size="small"
                    type="error"
                    aria-label="Delete secret"
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
          <SecretItemsPanel
            v-if="isExpanded(row.id)"
            :secret-id="row.id"
            :parent-inactive="!row.active"
          />
        </template>
      </NCard>
    </div>

    <NDataTable
      v-else
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-class-name="rowClassName"
      :row-key="(row: SecretMain) => row.id"
      :pagination="false"
      :theme-overrides="{ tdColorHover: 'transparent' }"
      :expanded-row-keys="expandedKeys"
      @update:expanded-row-keys="
        (keys: (string | number)[]) => (expandedKeys = keys.map(String))
      "
    />

    <SecretDetailDrawer v-model:show="showDetail" :secret-id="detailId" />
    <SecretFormModal
      v-model:show="showForm"
      :secret="editing"
      :default-app-id="appId"
      :default-slug="rows.length === 0 ? 'main' : null"
      lock-app
      @saved="fetchSecrets"
    />
  </div>
</template>

<style scoped>
.secrets-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.secrets-section__actions {
  /* Keep the action row from collapsing when there are no secrets yet. */
  min-height: 34px;
}

.secret-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.secret-card__title {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-weight: 600;
  max-width: 100%;
}

/* Let a long slug wrap instead of overflowing the card (no horizontal scroll). */
.secret-card__title :deep(.n-button__content) {
  white-space: normal;
  word-break: break-word;
  text-align: left;
}

.secret-card__field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.secret-card__label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.secret-card__mono {
  cursor: pointer;
  word-break: break-all;
}

/*
 * Secret rows are tinted to read as group headers above their items.
 * The tint sits on the <tr> with transparent cells, so the opacity of inactive
 * cells below does not fade it.
 */
.secrets-section :deep(tr.row--parent) {
  background-color: v-bind('themeVars.hoverColor');
}

.secrets-section :deep(tr.row--parent > td) {
  background-color: transparent;
}

/* Code chips share the tint colour — lift them onto the card colour. */
.secrets-section :deep(tr.row--parent .n-text--code) {
  background-color: v-bind('themeVars.cardColor');
}

/* Inactive secrets are dimmed; the status tag and actions stay full-strength. */
.secrets-section :deep(tr.row--inactive > td) {
  opacity: 0.55;
}

.secrets-section :deep(tr.row--inactive > td.cell--full) {
  opacity: 1;
}

.secret-card--inactive :deep(.n-card-header__main),
.secret-card--inactive :deep(.n-card__content) {
  opacity: 0.55;
}
</style>
