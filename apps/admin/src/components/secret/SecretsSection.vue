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
import { deleteSecret, listSecrets } from '@/api/secret'
import type { SecretMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useClipboard } from '@/composables/useClipboard'
import { itemsRevealCommandKey } from '@/constants/injection'
import type { RevealCommand } from '@/constants/injection'
import { createSecretItemsStore, secretItemsKey } from '@/composables/useSecretItems'

import SecretDetailDrawer from '@/components/secret/SecretDetailDrawer.vue'
import SecretFormModal from '@/components/secret/SecretFormModal.vue'
import SecretItemsPanel from '@/components/secret/SecretItemsPanel.vue'

const props = defineProps<{
  /** Application whose secrets are shown. */
  appId: string
}>()

const message = useMessage()
const { copy } = useClipboard()
const { isMobile } = useBreakpoint()

// Auto-expand every secret on load when there are at most this many.
const AUTO_EXPAND_MAX = 5

const rows = ref<SecretMain[]>([])
const loading = ref(false)
const expandedKeys = ref<string[]>([])

function isExpanded(id: string): boolean {
  return expandedKeys.value.includes(id)
}

// Broadcast a one-shot reveal/hide command; each items panel applies it to its
// own per-row flags, so individual rows stay togglable afterwards.
const showAll = ref(false)
const revealCommand = ref<RevealCommand>({ action: 'hide', seq: 0 })
provide(itemsRevealCommandKey, revealCommand)

// Shared items cache for all panels: lets us load several secrets' items in a
// single request instead of one request per expanded panel.
const itemsStore = createSecretItemsStore()
provide(secretItemsKey, itemsStore)

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
    // Collapsing all secrets also hides all revealed values.
    expandedKeys.value = []
    broadcastReveal(false)
  } else {
    const ids = rows.value.map((r) => r.id)
    // One batch request for all secrets instead of one per opened panel.
    void itemsStore.prefetch(ids)
    expandedKeys.value = ids
  }
}

function toggleRevealAll(): void {
  broadcastReveal(!showAll.value)
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

const columns = computed<DataTableColumns<SecretMain>>(() => [
  {
    type: 'expand',
    renderExpand: (row) => h(SecretItemsPanel, { secretId: row.id }),
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
    expandedKeys.value = []
    itemsStore.reset()
    broadcastReveal(false)
    await fetchSecrets()
    // Few secrets → expand them all by default for quicker overview. Their
    // items are fetched in a single batch request (not one per panel).
    if (rows.value.length && rows.value.length <= AUTO_EXPAND_MAX) {
      const ids = rows.value.map((r) => r.id)
      void itemsStore.prefetch(ids)
      expandedKeys.value = ids
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
          <SecretItemsPanel v-if="isExpanded(row.id)" :secret-id="row.id" />
        </template>
      </NCard>
    </div>

    <NDataTable
      v-else
      :columns="columns"
      :data="rows"
      :loading="loading"
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
</style>
