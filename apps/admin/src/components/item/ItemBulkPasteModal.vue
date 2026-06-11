<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NButton,
  NInput,
  NModal,
  NSpace,
  NSwitch,
  NTable,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'

import { ApiError } from '@/api/http'
import { createItem, updateItem } from '@/api/item'
import type { ItemMain } from '@/api/types'
import { parseKeyValueText } from '@/utils/kvParse'

const props = defineProps<{
  show: boolean
  /** Secret the imported items will belong to. */
  secretId: string
  /** Existing items of the secret — used to detect key conflicts. */
  items: ItemMain[]
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const message = useMessage()

const text = ref('')
const overwrite = ref(false)
const submitting = ref(false)

const parsed = computed(() => parseKeyValueText(text.value))

interface PreviewRow {
  key: string
  value: string
  existing: ItemMain | null
}

const previewRows = computed<PreviewRow[]>(() => {
  const byKey = new Map(props.items.map((i) => [i.key, i]))
  return parsed.value.pairs.map((p) => ({
    ...p,
    existing: byKey.get(p.key) ?? null,
  }))
})

const importableCount = computed(
  () =>
    previewRows.value.filter((r) => !r.existing || overwrite.value).length,
)

function rowStatus(row: PreviewRow): { label: string; type: 'success' | 'warning' | 'default' } {
  if (!row.existing) return { label: 'New', type: 'success' }
  return overwrite.value
    ? { label: 'Overwrite', type: 'warning' }
    : { label: 'Skip', type: 'default' }
}

// Reset and try to pre-fill from the clipboard whenever the modal opens.
watch(
  () => props.show,
  async (show) => {
    if (!show) return
    text.value = ''
    overwrite.value = false
    try {
      text.value = await navigator.clipboard.readText()
    } catch {
      // Clipboard permission denied / unsupported — user pastes manually.
    }
  },
)

async function readClipboard(): Promise<void> {
  try {
    const value = await navigator.clipboard.readText()
    if (!value.trim()) {
      message.warning('Clipboard is empty')
      return
    }
    text.value = value
  } catch {
    message.error('Clipboard unavailable')
  }
}

function close(): void {
  emit('update:show', false)
}

async function submit(): Promise<void> {
  submitting.value = true
  let created = 0
  let updated = 0
  let skipped = 0
  const failed: string[] = []

  for (const row of previewRows.value) {
    if (row.existing && !overwrite.value) {
      skipped++
      continue
    }
    try {
      if (row.existing) {
        // Imported values are plain text — reset any previous file payload.
        await updateItem(row.existing.id, {
          value: row.value,
          value_format: 'text',
          encoding: 'plain',
          file_name: '',
          content_type: '',
        })
        updated++
      } else {
        await createItem({
          secret_id: props.secretId,
          key: row.key,
          value: row.value,
          value_format: 'text',
          encoding: 'plain',
          description: '',
          active: true,
        })
        created++
      }
    } catch (error) {
      failed.push(
        error instanceof ApiError ? `${row.key}: ${error.message}` : row.key,
      )
    }
  }
  submitting.value = false

  if (created || updated) emit('saved')
  if (failed.length) {
    message.warning(
      `Imported ${created + updated} of ${created + updated + failed.length}, failed: ${failed.join(', ')}`,
    )
    return
  }
  const parts = [`${created} created`]
  if (updated) parts.push(`${updated} updated`)
  if (skipped) parts.push(`${skipped} skipped`)
  message.success(`Items imported: ${parts.join(', ')}`)
  close()
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Paste items"
    style="max-width: 680px"
    :mask-closable="!submitting"
    @update:show="emit('update:show', $event)"
  >
    <NSpace vertical :size="12">
      <div class="paste-bar">
        <NText depth="3" style="font-size: 12px">
          Paste key/value pairs — <code>key: value</code>, <code>key=value</code>,
          one per line or separated by <code>,</code> / <code>;</code>,
          .env (<code>export</code>, quotes, <code>#</code> comments),
          JSON object or spreadsheet columns.
        </NText>
        <NButton size="small" tertiary @click="readClipboard">
          Read clipboard
        </NButton>
      </div>

      <NInput
        v-model:value="text"
        type="textarea"
        class="paste-input"
        placeholder="DB_HOST=localhost&#10;DB_PASSWORD=secret&#10;api_key: abc123"
        :autosize="{ minRows: 5, maxRows: 12 }"
        :disabled="submitting"
      />

      <NText v-if="parsed.invalid.length" type="warning" style="font-size: 12px">
        Could not parse: {{ parsed.invalid.join(' · ') }}
      </NText>
      <NText v-if="parsed.duplicates.length" type="warning" style="font-size: 12px">
        Duplicate keys (last value wins): {{ parsed.duplicates.join(', ') }}
      </NText>

      <template v-if="previewRows.length">
        <div class="paste-preview">
          <NTable size="small" :bordered="false" :single-line="false">
            <thead>
              <tr>
                <th>Key</th>
                <th>Value</th>
                <th style="width: 100px">Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in previewRows" :key="row.key">
                <td class="paste-cell">{{ row.key }}</td>
                <td class="paste-cell">{{ row.value }}</td>
                <td>
                  <NTag size="small" :type="rowStatus(row).type">
                    {{ rowStatus(row).label }}
                  </NTag>
                </td>
              </tr>
            </tbody>
          </NTable>
        </div>

        <NSpace align="center" :size="8">
          <NSwitch v-model:value="overwrite" size="small" :disabled="submitting" />
          <NText style="font-size: 13px">Overwrite existing keys</NText>
        </NSpace>
      </template>
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton
          type="primary"
          :loading="submitting"
          :disabled="!importableCount"
          @click="submit"
        >
          Import {{ importableCount || '' }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.paste-bar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.paste-input :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.paste-preview {
  max-height: 260px;
  overflow: auto;
}

.paste-cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
