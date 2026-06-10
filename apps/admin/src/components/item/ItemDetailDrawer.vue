<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NButton,
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'

import { ApiError } from '@/api/http'
import { getItem } from '@/api/item'
import type { ItemMain } from '@/api/types'
import { useSecretOptions } from '@/composables/useSecretOptions'
import ValueEditor from '@/components/common/ValueEditor.vue'
import { base64ByteSize, downloadBase64, formatBytes } from '@/utils/binary'

const props = defineProps<{
  show: boolean
  /** Id of the item to display; details are (re)fetched when it changes. */
  itemId: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const message = useMessage()
const { nameOf, ensure } = useSecretOptions()

const loading = ref(false)
const item = ref<ItemMain | null>(null)
const revealed = ref(false)
// Local copy so "Decode base64" can transform the shown value non-destructively.
const displayValue = ref('')

const valueFormat = computed<'text' | 'yaml' | 'json'>(() =>
  item.value?.value_format === 'yaml' || item.value?.value_format === 'json'
    ? item.value.value_format
    : 'text',
)

const isFile = computed(() => item.value?.encoding === 'base64')
const fileSize = computed(() =>
  item.value ? formatBytes(base64ByteSize(item.value.value)) : '',
)

function downloadFile(): void {
  if (!item.value) return
  try {
    downloadBase64(item.value.value, item.value.file_name || item.value.key, item.value.content_type)
  } catch {
    message.error('Failed to download file')
  }
}

watch(
  () => [props.show, props.itemId] as const,
  async ([show, id]) => {
    if (!show || !id) return
    loading.value = true
    item.value = null
    revealed.value = false
    displayValue.value = ''
    try {
      const loaded = await getItem(id)
      item.value = loaded
      displayValue.value = loaded.value
      await ensure(loaded.secret_id)
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : 'Failed to load')
      emit('update:show', false)
    } finally {
      loading.value = false
    }
  },
)

async function copyValue(): Promise<void> {
  try {
    await navigator.clipboard.writeText(displayValue.value)
    message.success('Value copied')
  } catch {
    message.error('Clipboard unavailable')
  }
}

function decodeBase64(): void {
  try {
    const binary = atob(displayValue.value.trim())
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
    displayValue.value = new TextDecoder().decode(bytes)
    revealed.value = true
    message.success('Decoded from base64')
  } catch {
    message.error('Invalid base64 value')
  }
}

function formatDate(value: string | undefined): string {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}
</script>

<template>
  <NDrawer
    :show="show"
    :width="560"
    placement="right"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent title="Item details" closable>
      <NSpin :show="loading">
        <template v-if="item">
          <NDescriptions :column="1" label-placement="top" bordered size="small">
            <NDescriptionsItem label="ID">
              <NText code>{{ item.id }}</NText>
            </NDescriptionsItem>
            <NDescriptionsItem label="Key">
              {{ item.key }}
            </NDescriptionsItem>
            <NDescriptionsItem label="Secret">
              <NTag size="small" type="info">{{ nameOf(item.secret_id) }}</NTag>
            </NDescriptionsItem>
            <NDescriptionsItem label="Status">
              <NTag :type="item.active ? 'success' : 'default'" size="small">
                {{ item.active ? 'Active' : 'Inactive' }}
              </NTag>
            </NDescriptionsItem>
            <NDescriptionsItem label="Description">
              {{ item.description || '—' }}
            </NDescriptionsItem>
            <NDescriptionsItem label="Created">
              {{ formatDate(item.created_at) }}
            </NDescriptionsItem>
            <NDescriptionsItem label="Updated">
              {{ formatDate(item.updated_at) }}
            </NDescriptionsItem>
          </NDescriptions>

          <div class="value-section">
            <div class="value-section__bar">
              <NText depth="3" style="font-size: 12px">Value</NText>
              <NSpace v-if="isFile" :size="8">
                <NButton size="tiny" tertiary @click="downloadFile">Download</NButton>
              </NSpace>
              <NSpace v-else :size="8">
                <NButton size="tiny" tertiary @click="revealed = !revealed">
                  {{ revealed ? 'Hide' : 'Show' }}
                </NButton>
                <NButton size="tiny" tertiary @click="decodeBase64">Decode</NButton>
                <NButton size="tiny" tertiary @click="copyValue">Copy</NButton>
              </NSpace>
            </div>

            <div v-if="isFile" class="value-file">
              <NText class="value-file__name">{{ item.file_name || 'file' }}</NText>
              <NTag size="tiny" :bordered="false">{{ fileSize }}</NTag>
              <NTag
                v-if="item.content_type"
                size="tiny"
                :bordered="false"
                type="info"
              >
                {{ item.content_type }}
              </NTag>
            </div>

            <template v-else>
              <div v-if="revealed" class="value-box">
                <span class="value-box__type">{{ item.value_format || 'text' }}</span>
                <ValueEditor
                  :value="displayValue"
                  :format="valueFormat"
                  readonly
                  min-height="80px"
                  max-height="60vh"
                />
              </div>
              <NText v-else code class="value-section__masked">••••••••</NText>
            </template>
          </div>
        </template>
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.value-section {
  margin-top: 16px;
}

.value-section__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.value-box {
  position: relative;
}

.value-box__type {
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

.value-section__masked {
  display: block;
  padding: 8px 0;
}

.value-file {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 12px;
  border: 1px dashed rgba(128, 128, 128, 0.4);
  border-radius: 6px;
}

.value-file__name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
