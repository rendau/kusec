<script setup lang="ts">
import { inject, onMounted, ref, watch } from 'vue'
import type { Ref } from 'vue'
import {
  NButton,
  NFlex,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'

import { ApiError } from '@/api/http'
import { deleteItem, listItems } from '@/api/item'
import type { ItemMain } from '@/api/types'

import ItemBulkPasteModal from '@/components/item/ItemBulkPasteModal.vue'
import ItemDetailDrawer from '@/components/item/ItemDetailDrawer.vue'
import ItemFormModal from '@/components/item/ItemFormModal.vue'
import ValueEditor from '@/components/common/ValueEditor.vue'
import { base64ByteSize, downloadBase64, formatBytes } from '@/utils/binary'

function valueFormat(row: ItemMain): 'text' | 'yaml' | 'json' {
  return row.value_format === 'yaml' || row.value_format === 'json'
    ? row.value_format
    : 'text'
}

function isFileRow(row: ItemMain): boolean {
  return row.encoding === 'base64'
}

function fileSize(row: ItemMain): string {
  return formatBytes(base64ByteSize(row.value))
}

function downloadFile(row: ItemMain): void {
  try {
    downloadBase64(row.value, row.file_name || row.key, row.content_type)
  } catch {
    message.error('Failed to download file')
  }
}

const props = defineProps<{
  secretId: string
}>()

const message = useMessage()

const rows = ref<ItemMain[]>([])
const loading = ref(false)

// Ids whose value block is revealed (shown under the row).
const opened = ref(new Set<string>())

function isOpen(id: string): boolean {
  return opened.value.has(id)
}

function toggleReveal(id: string): void {
  const next = new Set(opened.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  opened.value = next
}

// Workspace "Show all / Hide all" broadcasts a command that sets our per-row
// flags — rows stay individually togglable afterwards.
interface RevealCommand {
  action: 'show' | 'hide'
  seq: number
}
const revealCommand = inject<Ref<RevealCommand>>(
  'itemsRevealCommand',
  ref({ action: 'hide', seq: 0 }),
)

function applyRevealCommand(): void {
  opened.value =
    revealCommand.value.action === 'show'
      ? new Set(rows.value.map((r) => r.id))
      : new Set()
}

watch(() => revealCommand.value.seq, applyRevealCommand)

const detailId = ref<string | null>(null)
const showDetail = ref(false)

const editing = ref<ItemMain | null>(null)
const showForm = ref(false)

const showPaste = ref(false)

async function fetchItems(): Promise<void> {
  loading.value = true
  try {
    // Scoped by secret_id → backend returns all items, no pagination needed.
    const rep = await listItems({ secret_id: props.secretId })
    rows.value = rep.results ?? []
    applyRevealCommand()
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to load items')
  } finally {
    loading.value = false
  }
}

async function copyValue(value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value)
    message.success('Value copied')
  } catch {
    message.error('Clipboard unavailable')
  }
}

// Decode a base64 value and copy the result to the clipboard.
async function decodeBase64ToClipboard(value: string): Promise<void> {
  let decoded: string
  try {
    const binary = atob(value.trim())
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
    decoded = new TextDecoder().decode(bytes)
  } catch {
    message.error('Invalid base64 value')
    return
  }
  try {
    await navigator.clipboard.writeText(decoded)
    message.success('Decoded value copied')
  } catch {
    message.error('Clipboard unavailable')
  }
}

function openDetail(row: ItemMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: ItemMain): void {
  editing.value = row
  showForm.value = true
}

async function removeItem(row: ItemMain): Promise<void> {
  try {
    await deleteItem(row.id)
    message.success('Item deleted')
    await fetchItems()
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to delete item')
  }
}

onMounted(fetchItems)
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
        <NButton size="tiny" @click="showPaste = true">Paste items</NButton>
        <NButton size="tiny" type="primary" @click="openCreate">New item</NButton>
      </NSpace>
    </NFlex>

    <NSpin :show="loading">
      <div v-if="!rows.length" class="items-empty">
        <NText depth="3">No items</NText>
      </div>

      <div v-else class="items">
        <div class="items__head">Key</div>
        <div class="items__head">Value</div>
        <div class="items__head">Status</div>
        <div class="items__head">Actions</div>

        <template v-for="(row, i) in rows" :key="row.id">
          <div v-if="i > 0" class="items__sep" />

          <div class="items__cell items__key">
            <NButton text type="primary" @click="openDetail(row)">
              {{ row.key }}
            </NButton>
          </div>

          <div class="items__cell">
            <NSpace :size="6" align="center" :wrap-item="false">
              <template v-if="isFileRow(row)">
                <NText depth="3" class="items__file-name">
                  {{ row.file_name || 'file' }}
                </NText>
                <NTag size="tiny" :bordered="false">{{ fileSize(row) }}</NTag>
                <NButton
                  text
                  type="primary"
                  size="tiny"
                  @click="downloadFile(row)"
                >
                  Download
                </NButton>
              </template>
              <template v-else>
                <NButton
                  text
                  type="primary"
                  size="tiny"
                  @click="toggleReveal(row.id)"
                >
                  {{ isOpen(row.id) ? 'Hide' : 'Show' }}
                </NButton>
                <NButton
                  text
                  type="primary"
                  size="tiny"
                  @click="copyValue(row.value)"
                >
                  Copy
                </NButton>
                <NButton
                  text
                  type="primary"
                  size="tiny"
                  @click="decodeBase64ToClipboard(row.value)"
                >
                  Decode
                </NButton>
              </template>
            </NSpace>
          </div>

          <div class="items__cell">
            <NTag :type="row.active ? 'success' : 'default'" size="small">
              {{ row.active ? 'Active' : 'Inactive' }}
            </NTag>
          </div>

          <div class="items__cell">
            <NSpace :size="8">
              <NButton size="tiny" @click="openEdit(row)">Edit</NButton>
              <NPopconfirm @positive-click="removeItem(row)">
                <template #trigger>
                  <NButton size="tiny" type="error" secondary>Delete</NButton>
                </template>
                Delete "{{ row.key }}"?
              </NPopconfirm>
            </NSpace>
          </div>

          <Transition name="value">
            <div v-if="!isFileRow(row) && isOpen(row.id)" class="items__value">
              <span class="items__value-type">{{ row.value_format || 'text' }}</span>
              <ValueEditor
                :value="row.value"
                :format="valueFormat(row)"
                readonly
                min-height="0"
                max-height="320px"
              />
            </div>
          </Transition>
        </template>
      </div>
    </NSpin>

    <ItemDetailDrawer v-model:show="showDetail" :item-id="detailId" />
    <ItemBulkPasteModal
      v-model:show="showPaste"
      :secret-id="secretId"
      :items="rows"
      @saved="fetchItems"
    />
    <ItemFormModal
      v-model:show="showForm"
      :item="editing"
      :default-secret-id="secretId"
      lock-secret
      @saved="fetchItems"
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
  grid-template-columns: minmax(140px, 1.2fr) minmax(220px, 2fr) 100px 170px;
  column-gap: 12px;
  row-gap: 4px;
  align-items: center;
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

/* Indent the first column so items read as children of the secret. */
.items__head:first-child,
.items__key {
  padding-left: 20px;
}

.items__key {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Value block: starts at the Value column and spans to the end (not under Key). */
.items__value {
  position: relative;
  grid-column: 2 / -1;
  margin: 2px 0 6px;
}

.items__value-type {
  position: absolute;
  top: 6px;
  right: 8px;
  z-index: 1;
  padding: 2px 7px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1.4;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color: #18a058;
  background: rgba(24, 160, 88, 0.14);
  border-radius: 999px;
  pointer-events: none;
}

/* Reveal animation. */
.value-enter-active,
.value-leave-active {
  overflow: hidden;
  transition:
    opacity 0.2s ease,
    max-height 0.2s ease,
    transform 0.2s ease;
}

.value-enter-from,
.value-leave-to {
  opacity: 0;
  max-height: 0;
  transform: translateY(-4px);
}

.value-enter-to,
.value-leave-from {
  opacity: 1;
  max-height: 360px;
}

@media (prefers-reduced-motion: reduce) {
  .value-enter-active,
  .value-leave-active {
    transition: none;
  }
}

</style>
