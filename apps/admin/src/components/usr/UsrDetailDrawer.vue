<script setup lang="ts">
import {
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NSpin,
  NTag,
  NText,
} from 'naive-ui'

import { getUser } from '@/api/usr'
import { useDrawerResource } from '@/composables/useDrawerResource'

const props = defineProps<{
  show: boolean
  /** Id of the user to display; details are (re)fetched when it changes. */
  userId: number | string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { loading, item: user } = useDrawerResource({
  show: () => props.show,
  id: () => props.userId,
  fetch: getUser,
  onError: () => emit('update:show', false),
})
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
