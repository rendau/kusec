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
import { getApp } from '@/api/app'
import type { AppMain } from '@/api/types'

const props = defineProps<{
  show: boolean
  /** Id of the app to display; details are (re)fetched when it changes. */
  appId: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const message = useMessage()

const loading = ref(false)
const app = ref<AppMain | null>(null)

watch(
  () => [props.show, props.appId] as const,
  async ([show, id]) => {
    if (!show || !id) return
    loading.value = true
    app.value = null
    try {
      app.value = await getApp(id)
    } catch (error) {
      const reason = error instanceof ApiError ? error.message : 'Failed to load'
      message.error(reason)
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
    <NDrawerContent title="Application details" closable>
      <NSpin :show="loading">
        <NDescriptions
          v-if="app"
          :column="1"
          label-placement="top"
          bordered
          size="small"
        >
          <NDescriptionsItem label="ID">
            <NText code>{{ app.id }}</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="Name">
            {{ app.name }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Namespace">
            <NTag size="small">{{ app.namespace }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <NTag :type="app.active ? 'success' : 'default'" size="small">
              {{ app.active ? 'Active' : 'Inactive' }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Description">
            {{ app.description || '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Created">
            {{ formatDate(app.created_at) }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Updated">
            {{ formatDate(app.updated_at) }}
          </NDescriptionsItem>
        </NDescriptions>
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>
