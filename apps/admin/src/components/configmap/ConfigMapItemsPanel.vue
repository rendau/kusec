<script setup lang="ts">
import { computed, inject, onMounted, ref } from 'vue'
import {
  NButton,
  NFlex,
  NIcon,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag,
  NText,
  NTooltip,
  useMessage,
} from 'naive-ui'
import { Binary, Clipboard, Copy, Download, Pencil, Plus, Trash } from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import { deleteConfigItem } from '@/api/configitem'
import type { ConfigItemMain } from '@/api/types'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useClipboard } from '@/composables/useClipboard'
import { createConfigItemsStore, configItemsKey } from '@/composables/useConfigItems'

import ConfigItemBulkPasteModal from '@/components/configitem/ConfigItemBulkPasteModal.vue'
import ConfigItemDetailDrawer from '@/components/configitem/ConfigItemDetailDrawer.vue'
import ConfigItemFormModal from '@/components/configitem/ConfigItemFormModal.vue'
import ValueEditor from '@/components/common/ValueEditor.vue'
import ValueFormatChip from '@/components/common/ValueFormatChip.vue'
import {
  base64ByteSize,
  base64ToText,
  downloadBase64,
  formatBytes,
  isProbablyBase64,
} from '@/utils/binary'
import { normalizeValueFormat } from '@/utils/format'

const props = defineProps<{
  configmapId: string
}>()

const message = useMessage()
const { copy } = useClipboard()
const { isMobile } = useBreakpoint()

// Shared cache from the workspace (batches the items request across panels);
// falls back to a private store when the panel is used standalone.
const store = inject(configItemsKey, null) ?? createConfigItemsStore()

const rows = computed<ConfigItemMain[]>(
  () => store.itemsByConfigMap.value[props.configmapId] ?? [],
)
const loading = computed(
  () =>
    store.itemsByConfigMap.value[props.configmapId] === undefined &&
    !!store.loadingByConfigMap.value[props.configmapId],
)

function refresh(): Promise<void> {
  return store.reload(props.configmapId)
}

function isFileRow(row: ConfigItemMain): boolean {
  return row.encoding === 'base64'
}

function fileSize(row: ConfigItemMain): string {
  return formatBytes(base64ByteSize(row.value))
}

function downloadFile(row: ConfigItemMain): void {
  try {
    downloadBase64(row.value, row.file_name || row.key, row.content_type)
  } catch {
    message.error('Failed to download file')
  }
}

const detailId = ref<string | null>(null)
const showDetail = ref(false)

const editing = ref<ConfigItemMain | null>(null)
const showForm = ref(false)

const showPaste = ref(false)

/** Decode a base64 value and copy the result (offered for base64-looking values). */
async function copyDecoded(value: string): Promise<void> {
  try {
    await copy(base64ToText(value.trim()), 'Decoded value copied')
  } catch {
    message.error('Invalid base64 value')
  }
}

function openDetail(row: ConfigItemMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: ConfigItemMain): void {
  editing.value = row
  showForm.value = true
}

async function removeItem(row: ConfigItemMain): Promise<void> {
  try {
    await deleteConfigItem(row.id)
    message.success('Config item deleted')
    await refresh()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to delete config item'))
  }
}

// Load this config map's items if the shared store hasn't already (manual
// expand); a no-op when the workspace prefetched them in a batch request.
onMounted(() => {
  void store.ensure(props.configmapId)
})
</script>

<template>
  <div class="items-panel">
    <NFlex
      justify="space-between"
      align="center"
      style="margin-bottom: 8px; padding-left: 20px"
    >
      <NText depth="3" style="font-size: 12px">Items</NText>
      <NSpace :size="8">
        <NButton size="tiny" @click="showPaste = true">
          <template #icon>
            <NIcon :component="Clipboard" />
          </template>
          Paste items
        </NButton>
        <NButton size="tiny" type="primary" @click="openCreate">
          <template #icon>
            <NIcon :component="Plus" />
          </template>
          New item
        </NButton>
      </NSpace>
    </NFlex>

    <NSpin :show="loading">
      <div v-if="!rows.length" class="items-empty">
        <NText depth="3">No items</NText>
      </div>

      <div v-else-if="!isMobile" class="items">
        <div class="items__head">Key</div>
        <div class="items__head">Value</div>
        <div class="items__head">Status</div>
        <div class="items__head">Actions</div>

        <template v-for="(row, i) in rows" :key="row.id">
          <div v-if="i > 0" class="items__sep" />

          <div class="items__cell items__key">
            <NButton
              text
              type="primary"
              class="items__key-btn"
              @click="openDetail(row)"
            >
              {{ row.key }}
            </NButton>
          </div>

          <div class="items__cell">
            <NSpace :size="4" align="center" :wrap-item="false">
              <template v-if="isFileRow(row)">
                <NText depth="3" class="items__file-name">
                  {{ row.file_name || 'file' }}
                </NText>
                <NTag size="tiny" :bordered="false">{{ fileSize(row) }}</NTag>
                <NTooltip>
                  <template #trigger>
                    <NButton
                      quaternary
                      circle
                      size="tiny"
                      aria-label="Download file"
                      @click="downloadFile(row)"
                    >
                      <template #icon>
                        <NIcon :component="Download" />
                      </template>
                    </NButton>
                  </template>
                  Download
                </NTooltip>
              </template>
              <template v-else>
                <NTooltip>
                  <template #trigger>
                    <NButton
                      quaternary
                      circle
                      size="tiny"
                      aria-label="Copy value"
                      @click="copy(row.value)"
                    >
                      <template #icon>
                        <NIcon :component="Copy" />
                      </template>
                    </NButton>
                  </template>
                  Copy value
                </NTooltip>
                <NTooltip v-if="isProbablyBase64(row.value)">
                  <template #trigger>
                    <NButton
                      quaternary
                      circle
                      size="tiny"
                      aria-label="Copy decoded value"
                      @click="copyDecoded(row.value)"
                    >
                      <template #icon>
                        <NIcon :component="Binary" />
                      </template>
                    </NButton>
                  </template>
                  Copy decoded (base64)
                </NTooltip>
              </template>
            </NSpace>
          </div>

          <div class="items__cell">
            <NTag :type="row.active ? 'success' : 'default'" size="small">
              {{ row.active ? 'Active' : 'Inactive' }}
            </NTag>
          </div>

          <div class="items__cell">
            <NSpace :size="4" :wrap-item="false">
              <NTooltip>
                <template #trigger>
                  <NButton
                    quaternary
                    circle
                    size="tiny"
                    aria-label="Edit item"
                    @click="openEdit(row)"
                  >
                    <template #icon>
                      <NIcon :component="Pencil" />
                    </template>
                  </NButton>
                </template>
                Edit
              </NTooltip>
              <NPopconfirm @positive-click="removeItem(row)">
                <template #trigger>
                  <NButton
                    quaternary
                    circle
                    size="tiny"
                    type="error"
                    aria-label="Delete item"
                  >
                    <template #icon>
                      <NIcon :component="Trash" />
                    </template>
                  </NButton>
                </template>
                Delete "{{ row.key }}"?
              </NPopconfirm>
            </NSpace>
          </div>

          <div v-if="!isFileRow(row)" class="items__value">
            <ValueFormatChip :format="row.value_format" />
            <ValueEditor
              :value="row.value"
              :format="normalizeValueFormat(row.value_format)"
              readonly
              min-height="0"
              max-height="320px"
            />
          </div>
        </template>
      </div>

      <!-- Mobile: stacked item cards (no horizontal scroll). -->
      <div v-else class="items-m">
        <div v-for="row in rows" :key="row.id" class="item-m">
          <div class="item-m__top">
            <NButton
              text
              type="primary"
              class="items__key-btn item-m__key"
              @click="openDetail(row)"
            >
              {{ row.key }}
            </NButton>
            <NTag :type="row.active ? 'success' : 'default'" size="tiny">
              {{ row.active ? 'Active' : 'Inactive' }}
            </NTag>
          </div>

          <div class="item-m__actions">
            <template v-if="isFileRow(row)">
              <NText depth="3" class="items__file-name">
                {{ row.file_name || 'file' }}
              </NText>
              <NTag size="tiny" :bordered="false">{{ fileSize(row) }}</NTag>
              <NButton
                quaternary
                circle
                size="tiny"
                aria-label="Download file"
                @click="downloadFile(row)"
              >
                <template #icon>
                  <NIcon :component="Download" />
                </template>
              </NButton>
            </template>
            <template v-else>
              <NButton
                quaternary
                circle
                size="tiny"
                aria-label="Copy value"
                @click="copy(row.value)"
              >
                <template #icon>
                  <NIcon :component="Copy" />
                </template>
              </NButton>
              <NButton
                v-if="isProbablyBase64(row.value)"
                quaternary
                circle
                size="tiny"
                aria-label="Copy decoded value"
                @click="copyDecoded(row.value)"
              >
                <template #icon>
                  <NIcon :component="Binary" />
                </template>
              </NButton>
            </template>

            <span class="item-m__spacer" />

            <NButton
              quaternary
              circle
              size="tiny"
              aria-label="Edit item"
              @click="openEdit(row)"
            >
              <template #icon>
                <NIcon :component="Pencil" />
              </template>
            </NButton>
            <NPopconfirm @positive-click="removeItem(row)">
              <template #trigger>
                <NButton
                  quaternary
                  circle
                  size="tiny"
                  type="error"
                  aria-label="Delete item"
                >
                  <template #icon>
                    <NIcon :component="Trash" />
                  </template>
                </NButton>
              </template>
              Delete "{{ row.key }}"?
            </NPopconfirm>
          </div>

          <div v-if="!isFileRow(row)" class="item-m__value">
            <ValueFormatChip :format="row.value_format" />
            <ValueEditor
              :value="row.value"
              :format="normalizeValueFormat(row.value_format)"
              readonly
              min-height="0"
              max-height="320px"
            />
          </div>
        </div>
      </div>
    </NSpin>

    <ConfigItemDetailDrawer v-model:show="showDetail" :item-id="detailId" />
    <ConfigItemBulkPasteModal
      v-model:show="showPaste"
      :configmap-id="configmapId"
      :items="rows"
      @saved="refresh"
    />
    <ConfigItemFormModal
      v-model:show="showForm"
      :item="editing"
      :default-config-map-id="configmapId"
      lock-config-map
      @saved="refresh"
    />
  </div>
</template>

<style scoped>
.items-panel {
  padding: 4px 8px 8px;
}

.items-empty {
  padding: 16px 0;
}

.items__file-name {
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Divider between item rows. */
.items__sep {
  grid-column: 1 / -1;
  height: 0;
  margin: 4px 0;
  border-top: 1px solid rgba(128, 128, 128, 0.18);
}

.items {
  display: grid;
  grid-template-columns: minmax(140px, 1.2fr) minmax(220px, 2fr) 100px 110px;
  column-gap: 12px;
  row-gap: 4px;
  align-items: center;
}

/* Mobile: each item is a stacked card — no horizontal scroll. */
.items-m {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.item-m {
  padding: 8px 4px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.18);
}

.item-m:last-child {
  border-bottom: none;
}

.item-m__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.item-m__key {
  flex: 1;
  min-width: 0;
}

/* Truncate long keys instead of overflowing the card (full key in details). */
.item-m__key :deep(.n-button__content) {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-m__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
}

.item-m__spacer {
  flex: 1;
}

.item-m__value {
  position: relative;
  margin: 8px 0 2px;
}

.items__head {
  font-size: 12px;
  font-weight: 600;
  opacity: 0.55;
  padding: 4px 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
}

.items__cell {
  min-width: 0;
  padding: 2px 0;
}

/* Indent the first column so items read as children of the config map. */
.items__head:first-child,
.items__key {
  padding-left: 20px;
}

.items__key {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Item keys are env-style identifiers — render them in monospace. */
.items__key-btn {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
}

/* Value block: starts at the Value column and spans to the end (not under Key). */
.items__value {
  position: relative;
  grid-column: 2 / -1;
  margin: 2px 0 6px;
}
</style>
