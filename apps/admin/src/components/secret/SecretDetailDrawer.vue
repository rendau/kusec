<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'

import { ApiError } from '@/api/http'
import { getSecret } from '@/api/secret'
import type { SecretMain } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'

const props = defineProps<{
  show: boolean
  /** Id of the secret to display; details are (re)fetched when it changes. */
  secretId: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const message = useMessage()
const { nameOf, ensure } = useAppOptions()

const loading = ref(false)
const secret = ref<SecretMain | null>(null)

watch(
  () => [props.show, props.secretId] as const,
  async ([show, id]) => {
    if (!show || !id) return
    loading.value = true
    secret.value = null
    try {
      const loaded = await getSecret(id)
      secret.value = loaded
      await ensure(loaded.app_id)
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : 'Failed to load')
      emit('update:show', false)
    } finally {
      loading.value = false
    }
  },
)

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
    <NDrawerContent title="Secret details" closable>
      <NSpin :show="loading">
        <NDescriptions
          v-if="secret"
          :column="1"
          label-placement="top"
          bordered
          size="small"
        >
          <NDescriptionsItem label="ID">
            <NText code>{{ secret.id }}</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="Slug">
            {{ secret.slug_name }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Application">
            <NTag size="small" type="info">{{ nameOf(secret.app_id) }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <NTag :type="secret.active ? 'success' : 'default'" size="small">
              {{ secret.active ? 'Active' : 'Inactive' }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Description">
            {{ secret.description || '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Created">
            {{ formatDate(secret.created_at) }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Updated">
            {{ formatDate(secret.updated_at) }}
          </NDescriptionsItem>
        </NDescriptions>
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>
