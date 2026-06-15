<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NDropdown,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { FormRules } from 'naive-ui'

import { createItem, updateItem } from '@/api/item'
import type { ItemMain, ItemUpdateReq, ValueEncoding, ValueFormat } from '@/api/types'
import { useClipboard } from '@/composables/useClipboard'
import { useEntityForm } from '@/composables/useEntityForm'
import { useSecretOptions } from '@/composables/useSecretOptions'
import {
  base64ByteSize,
  base64ToText,
  bytesToBase64,
  downloadBase64,
  formatBytes,
  isProbablyText,
  textToBase64,
} from '@/utils/binary'
import { normalizeValueFormat, stripCommonIndent } from '@/utils/format'
import { randomHex } from '@/utils/random'

const MAX_FILE_BYTES = 5 * 1024 * 1024

// Lazy — pulls in the CodeMirror chunk only when the item form is opened.
const ValueEditor = defineAsyncComponent(
  () => import('@/components/common/ValueEditor.vue'),
)

const props = defineProps<{
  show: boolean
  /** The item being edited, or `null` when creating a new one. */
  item: ItemMain | null
  /** Pre-selected secret id when creating from within a secret context. */
  defaultSecretId?: string | null
  /** Lock the secret field (the item is bound to its secret). */
  lockSecret?: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const message = useMessage()
const { copy } = useClipboard()
const {
  options: secretOptions,
  loading: secretsLoading,
  search,
  ensure,
} = useSecretOptions()

// Editor mode for the value (persisted as `value_format`).
const valueFormat = ref<ValueFormat>('text')

interface FormModel {
  secret_id: string | null
  key: string
  value: string
  encoding: ValueEncoding
  file_name: string
  content_type: string
  description: string
  active: boolean
}

const model = reactive<FormModel>({
  secret_id: null,
  key: '',
  value: '',
  encoding: 'plain',
  file_name: '',
  content_type: '',
  description: '',
  active: true,
})

const fileInput = ref<HTMLInputElement | null>(null)
const isFile = computed(() => model.encoding === 'base64')
const fileSize = computed(() => formatBytes(base64ByteSize(model.value)))

const rules: FormRules = {
  secret_id: [
    {
      required: true,
      message: 'Secret is required',
      trigger: ['blur', 'change'],
      validator: (_rule, value: string | null) => value != null && value !== '',
    },
  ],
  // Validate on input/submit only — not on blur: clicking "Upload file" blurs
  // the key input, and a file fills the key from its name, so a blur error here
  // would flash spuriously right before the field gets populated.
  key: [{ required: true, message: 'Key is required', trigger: 'input' }],
}

/**
 * Значение к сохранению: для yaml/json убираем общий левый отступ
 * (вставка из вложенной секции другого файла). Файлы (base64) и
 * неструктурированный text не трогаем — там отступ может быть значимым.
 */
function preparedValue(): string {
  if (model.encoding === 'base64' || valueFormat.value === 'text') {
    return model.value
  }
  return stripCommonIndent(model.value)
}

const { formRef, submitting, isEdit, submit } = useEntityForm<ItemMain>({
  show: () => props.show,
  entity: () => props.item,
  seed: async (item) => {
    model.secret_id = item?.secret_id ?? props.defaultSecretId ?? null
    model.key = item?.key ?? ''
    model.value = item?.value ?? ''
    model.encoding = item?.encoding === 'base64' ? 'base64' : 'plain'
    model.file_name = item?.file_name ?? ''
    model.content_type = item?.content_type ?? ''
    model.description = item?.description ?? ''
    model.active = item?.active ?? true
    valueFormat.value = normalizeValueFormat(item?.value_format)
    if (model.secret_id) await ensure(model.secret_id)
  },
  create: () =>
    createItem({
      secret_id: model.secret_id as string,
      key: model.key,
      value: preparedValue(),
      value_format: valueFormat.value,
      encoding: model.encoding,
      file_name: model.file_name,
      content_type: model.content_type,
      description: model.description,
      active: model.active,
    }),
  update: (item) => {
    const update: ItemUpdateReq = {
      key: model.key,
      value: preparedValue(),
      value_format: valueFormat.value,
      encoding: model.encoding,
      file_name: model.file_name,
      content_type: model.content_type,
      description: model.description,
      active: model.active,
    }
    if (model.secret_id) update.secret_id = model.secret_id
    return updateItem(item.id, update)
  },
  messages: { created: 'Item created', updated: 'Item updated' },
  onSaved: () => {
    emit('saved')
    close()
  },
})

onMounted(() => {
  void search()
})

function close(): void {
  emit('update:show', false)
}

function encodeBase64(): void {
  if (!model.value) return
  model.value = textToBase64(model.value)
  message.success('Encoded to base64')
}

function decodeBase64(): void {
  if (!model.value) return
  try {
    model.value = base64ToText(model.value.trim())
    message.success('Decoded from base64')
  } catch {
    message.error('Invalid base64 value')
  }
}

function onBase64Select(key: string | number): void {
  if (key === 'encode') encodeBase64()
  else if (key === 'decode') decodeBase64()
}

// Варианты длины генерируемой строки (в hex-символах).
const generateOptions = [
  { label: '16 chars', key: 16 },
  { label: '24 chars', key: 24 },
  { label: '32 chars', key: 32 },
  { label: '48 chars', key: 48 },
  { label: '64 chars', key: 64 },
]

function onGenerateSelect(key: string | number): void {
  const chars = Number(key)
  model.value = randomHex(chars / 2)
  valueFormat.value = 'text'
  message.success(`Random value generated (${chars} chars)`)
}

const base64Options = [
  { label: 'Encode → base64', key: 'encode' },
  { label: 'Decode ← base64', key: 'decode' },
]

function pickFile(): void {
  fileInput.value?.click()
}

function formatFromName(name: string): ValueFormat {
  const ext = name.split('.').pop()?.toLowerCase()
  if (ext === 'yaml' || ext === 'yml') return 'yaml'
  if (ext === 'json') return 'json'
  return 'text'
}

async function onFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // allow re-selecting the same file
  if (!file) return
  if (file.size > MAX_FILE_BYTES) {
    message.error(`File is too large (max ${formatBytes(MAX_FILE_BYTES)})`)
    return
  }
  try {
    const bytes = new Uint8Array(await file.arrayBuffer())
    if (!model.key) model.key = file.name
    if (isProbablyText(bytes)) {
      // Text file (PEM/YAML/JSON/…) — keep editable & viewable as text.
      model.value = new TextDecoder().decode(bytes)
      model.encoding = 'plain'
      model.file_name = ''
      model.content_type = ''
      valueFormat.value = formatFromName(file.name)
      message.success('File loaded as text')
    } else {
      // Binary file — store base64, download-only.
      model.value = bytesToBase64(bytes)
      model.encoding = 'base64'
      model.file_name = file.name
      model.content_type = file.type
      message.success('Binary file attached')
    }
  } catch {
    message.error('Failed to read file')
  }
}

function removeFile(): void {
  model.value = ''
  model.encoding = 'plain'
  model.file_name = ''
  model.content_type = ''
}

function downloadFile(): void {
  try {
    downloadBase64(model.value, model.file_name || model.key, model.content_type)
  } catch {
    message.error('Failed to download file')
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit ? 'Edit item' : 'New item'"
    style="max-width: 680px"
    :mask-closable="!submitting"
    @update:show="emit('update:show', $event)"
  >
    <NForm
      ref="formRef"
      :model="model"
      :rules="rules"
      label-placement="top"
      :disabled="submitting"
    >
      <NFormItem label="Secret" path="secret_id">
        <NSelect
          v-model:value="model.secret_id"
          :options="secretOptions"
          :loading="secretsLoading"
          :disabled="lockSecret"
          :clearable="!lockSecret"
          filterable
          remote
          placeholder="Select a secret"
          @search="search"
        />
      </NFormItem>
      <NFormItem label="Key" path="key">
        <NInput v-model:value="model.key" placeholder="e.g. password" clearable />
      </NFormItem>
      <NFormItem label="Value" path="value">
        <div class="value-field">
          <input
            ref="fileInput"
            type="file"
            style="display: none"
            @change="onFileChange"
          />

          <!-- File / binary value -->
          <div v-if="isFile" class="value-file">
            <div class="value-file__info">
              <NText class="value-file__name">{{ model.file_name || 'file' }}</NText>
              <NTag size="tiny" :bordered="false">{{ fileSize }}</NTag>
              <NTag v-if="model.content_type" size="tiny" :bordered="false" type="info">
                {{ model.content_type }}
              </NTag>
            </div>
            <NSpace :size="8">
              <NButton size="small" tertiary @click="downloadFile">Download</NButton>
              <NButton size="small" tertiary @click="pickFile">Replace</NButton>
              <NButton size="small" tertiary type="error" @click="removeFile">
                Remove
              </NButton>
            </NSpace>
          </div>

          <!-- Text value -->
          <template v-else>
            <div class="value-field__bar">
              <NRadioGroup v-model:value="valueFormat" size="small">
                <NRadioButton value="text">Text</NRadioButton>
                <NRadioButton value="yaml">YAML</NRadioButton>
                <NRadioButton value="json">JSON</NRadioButton>
              </NRadioGroup>
              <NSpace :size="8">
                <NDropdown
                  trigger="click"
                  :options="generateOptions"
                  @select="onGenerateSelect"
                >
                  <NButton size="small" tertiary>Generate ▾</NButton>
                </NDropdown>
                <NButton size="small" tertiary @click="pickFile">Upload file</NButton>
                <NDropdown
                  trigger="click"
                  :options="base64Options"
                  :disabled="!model.value"
                  @select="onBase64Select"
                >
                  <NButton size="small" tertiary :disabled="!model.value">
                    base64 ▾
                  </NButton>
                </NDropdown>
                <NButton
                  size="small"
                  tertiary
                  :disabled="!model.value"
                  @click="copy(model.value)"
                >
                  Copy
                </NButton>
              </NSpace>
            </div>
            <ValueEditor v-model:value="model.value" :format="valueFormat" />
          </template>
        </div>
      </NFormItem>
      <NFormItem label="Description" path="description">
        <NInput
          v-model:value="model.description"
          type="textarea"
          placeholder="Optional description"
          :autosize="{ minRows: 2, maxRows: 5 }"
        />
      </NFormItem>
      <NFormItem label="Active" path="active">
        <NSwitch v-model:value="model.active" />
      </NFormItem>
    </NForm>

    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton type="primary" :loading="submitting" @click="submit">
          {{ isEdit ? 'Save' : 'Create' }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.value-field {
  width: 100%;
}

.value-field__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  /* Let the action buttons drop below the format switch on narrow screens. */
  flex-wrap: wrap;
}

.value-file {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px;
  border: 1px dashed rgba(128, 128, 128, 0.4);
  border-radius: 6px;
}

.value-file__info {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.value-file__name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
