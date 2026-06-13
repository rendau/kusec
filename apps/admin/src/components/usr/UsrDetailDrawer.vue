<script setup lang="ts">
import { watch } from 'vue'
import {
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NSpace,
  NSpin,
  NTag,
  NText,
} from 'naive-ui'

import { getUser } from '@/api/usr'
import { useAppOptions } from '@/composables/useAppOptions'
import { useDrawerResource } from '@/composables/useDrawerResource'

const props = defineProps<{
  show: boolean
  /** Id of the user to display; details are (re)fetched when it changes. */
  userId: number | string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { ensure, nameOf } = useAppOptions()

const { loading, item: user } = useDrawerResource({
  show: () => props.show,
  id: () => props.userId,
  fetch: getUser,
  onError: () => emit('update:show', false),
})

// Resolve app names for the access list whenever a user is loaded.
watch(user, (value) => {
  for (const id of value?.app_ids ?? []) void ensure(id)
})
</script>

<template>
  <NDrawer
    :show="show"
    width="min(420px, 100vw)"
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
          <NDescriptionsItem label="Application access">
            <NText v-if="user.is_admin || !user.app_ids?.length" :depth="3">
              All applications
            </NText>
            <NSpace v-else :size="4">
              <NTag v-for="id in user.app_ids" :key="id" size="small">
                {{ nameOf(id) }}
              </NTag>
            </NSpace>
          </NDescriptionsItem>
        </NDescriptions>
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>
