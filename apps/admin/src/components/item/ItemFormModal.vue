<script setup lang="ts">
import { defineAsyncComponent, onMounted, reactive, ref, watch } from 'vue'
import { computed } from 'vue'
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
import type { FormInst, FormRules } from 'naive-ui'

import { ApiError } from '@/api/http'
import { createItem, updateItem } from '@/api/item'
import type { ItemMain, ItemUpdateReq } from '@/api/types'
import { useSecretOptions } from '@/composables/useSecretOptions'
import {
  base64ByteSize,
  bytesToBase64,
  downloadBase64,
  formatBytes,
  isProbablyText,
} from '@/utils/binary'

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
const {
  options: secretOptions,
  loading: secretsLoading,
  search,
  ensure,
} = useSecretOptions()

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

// Editor mode for the value (persisted as `value_format`).
const valueFormat = ref<'text' | 'yaml' | 'json'>('text')

interface FormModel {
  secret_id: string | null
  key: string
  value: string
  encoding: 'plain' | 'base64'
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
  key: [{ required: true, message: 'Key is required', trigger: ['blur', 'input'] }],
}

const isEdit = () => props.item !== null

onMounted(() => {
  void search()
})

// Reset the form whenever the modal opens, seeding it from the edited item.
watch(
  () => props.show,
  async (show) => {
    if (!show) return
    const item = props.item
    model.secret_id = item?.secret_id ?? props.defaultSecretId ?? null
    model.key = item?.key ?? ''
    model.value = item?.value ?? ''
    model.encoding = item?.encoding === 'base64' ? 'base64' : 'plain'
    model.file_name = item?.file_name ?? ''
    model.content_type = item?.content_type ?? ''
    model.description = item?.description ?? ''
    model.active = item?.active ?? true
    valueFormat.value =
      item?.value_format === 'yaml' || item?.value_format === 'json'
        ? item.value_format
        : 'text'
    formRef.value?.restoreValidation()
    if (model.secret_id) await ensure(model.secret_id)
  },
)

function close(): void {
  emit('update:show', false)
}

async function copyValue(): Promise<void> {
  try {
    await navigator.clipboard.writeText(model.value)
    message.success('Value copied')
  } catch {
    message.error('Clipboard unavailable')
  }
}

// UTF-8-safe base64 (handles unicode/binary, unlike bare btoa/atob).
function encodeBase64(): void {
  if (!model.value) return
  const bytes = new TextEncoder().encode(model.value)
  let binary = ''
  bytes.forEach((b) => (binary += String.fromCharCode(b)))
  model.value = btoa(binary)
  message.success('Encoded to base64')
}

function decodeBase64(): void {
  if (!model.value) return
  try {
    const binary = atob(model.value.trim())
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
    model.value = new TextDecoder().decode(bytes)
    message.success('Decoded from base64')
  } catch {
    message.error('Invalid base64 value')
  }
}

function onBase64Select(key: string | number): void {
  if (key === 'encode') encodeBase64()
  else if (key === 'decode') decodeBase64()
}

const base64Options = [
  { label: 'Encode → base64', key: 'encode' },
  { label: 'Decode ← base64', key: 'decode' },
]

function pickFile(): void {
  fileInput.value?.click()
}

function formatFromName(name: string): 'text' | 'yaml' | 'json' {
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

async function submit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  submitting.value = true
  try {
    if (props.item) {
      const update: ItemUpdateReq = {
        key: model.key,
        value: model.value,
        value_format: valueFormat.value,
        encoding: model.encoding,
        file_name: model.file_name,
        content_type: model.content_type,
        description: model.description,
        active: model.active,
      }
      if (model.secret_id) update.secret_id = model.secret_id
      await updateItem(props.item.id, update)
      message.success('Item updated')
    } else {
      await createItem({
        secret_id: model.secret_id as string,
        key: model.key,
        value: model.value,
        value_format: valueFormat.value,
        encoding: model.encoding,
        file_name: model.file_name,
        content_type: model.content_type,
        description: model.description,
        active: model.active,
      })
      message.success('Item created')
    }
    emit('saved')
    close()
  } catch (error) {
    message.error(
      error instanceof ApiError ? error.message : 'Unexpected error, please try again',
    )
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit() ? 'Edit item' : 'New item'"
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
                  @click="copyValue"
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
          {{ isEdit() ? 'Save' : 'Create' }}
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
