<script setup lang="ts">
import { ref, watch } from 'vue'
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

watch(
  () => [props.show, props.itemId] as const,
  async ([show, id]) => {
    if (!show || !id) return
    loading.value = true
    item.value = null
    revealed.value = false
    try {
      const loaded = await getItem(id)
      item.value = loaded
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
  if (!item.value) return
  try {
    await navigator.clipboard.writeText(item.value.value)
    message.success('Value copied')
  } catch {
    message.error('Clipboard unavailable')
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
    :width="420"
    placement="right"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent title="Item details" closable>
      <NSpin :show="loading">
        <NDescriptions
          v-if="item"
          :column="1"
          label-placement="top"
          bordered
          size="small"
        >
          <NDescriptionsItem label="ID">
            <NText code>{{ item.id }}</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="Key">
            {{ item.key }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Secret">
            <NTag size="small" type="info">{{ nameOf(item.secret_id) }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Value">
            <NSpace align="center" :size="8" :wrap-item="false">
              <NText code style="word-break: break-all">
                {{ revealed ? item.value || '—' : '••••••••' }}
              </NText>
              <NButton text type="primary" size="tiny" @click="revealed = !revealed">
                {{ revealed ? 'Hide' : 'Show' }}
              </NButton>
              <NButton text type="primary" size="tiny" @click="copyValue">
                Copy
              </NButton>
            </NSpace>
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
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>
