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
import { getUser } from '@/api/usr'
import type { UsrMain } from '@/api/types'

const props = defineProps<{
  show: boolean
  /** Id of the user to display; details are (re)fetched when it changes. */
  userId: number | string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const message = useMessage()

const loading = ref(false)
const user = ref<UsrMain | null>(null)

watch(
  () => [props.show, props.userId] as const,
  async ([show, id]) => {
    if (!show || id == null) return
    loading.value = true
    user.value = null
    try {
      user.value = await getUser(id)
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : 'Failed to load')
      emit('update:show', false)
    } finally {
      loading.value = false
    }
  },
)
</script>

<template>
  <NDrawer
    :show="show"
    :width="420"
    placement="right"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent title="User details" closable>
      <NSpin :show="loading">
        <NDescriptions
          v-if="user"
          :column="1"
          label-placement="top"
          bordered
          size="small"
        >
          <NDescriptionsItem label="ID">
            <NText code>{{ user.id }}</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="Name">
            {{ user.name }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Username">
            {{ user.username }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Role">
            <NTag :type="user.is_admin ? 'success' : 'default'" size="small">
              {{ user.is_admin ? 'Administrator' : 'User' }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <NTag :type="user.active ? 'success' : 'default'" size="small">
              {{ user.active ? 'Active' : 'Inactive' }}
            </NTag>
          </NDescriptionsItem>
        </NDescriptions>
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>
