<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NDrawer,
  NDrawerContent,
  NSpace,
  NSpin,
  NTag,
  NText,
} from 'naive-ui'

import { listApiKeys } from '@/api/apikey'
import { getUser } from '@/api/usr'
import type { ApiKeyMain } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useDrawerResource } from '@/composables/useDrawerResource'
import { formatDate } from '@/utils/format'

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

// The user's API keys (the drawer is admin-only, so the owner filter is allowed).
const apiKeys = ref<ApiKeyMain[]>([])
const apiKeysLoading = ref(false)

// Resolve app names + load the key list whenever a user is loaded.
watch(user, async (value) => {
  for (const id of value?.app_ids ?? []) void ensure(id)

  apiKeys.value = []
  if (!value) return
  apiKeysLoading.value = true
  try {
    const rep = await listApiKeys({
      list_params: { page: 0, page_size: 100 },
      usr_id: String(value.id),
    })
    apiKeys.value = rep.results ?? []
  } catch {
    // секция вспомогательная — ошибку не эскалируем
  } finally {
    apiKeysLoading.value = false
  }
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
          <NDescriptionsItem label="Two-factor auth">
            <NTag :type="user.totp_enabled ? 'success' : 'default'" size="small">
              {{ user.totp_enabled ? 'Enabled' : 'Disabled' }}
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

        <template v-if="user">
          <NDivider style="margin: 16px 0 12px">
            API keys{{ apiKeys.length ? ` (${apiKeys.length})` : '' }}
          </NDivider>
          <NSpin :show="apiKeysLoading">
            <NText v-if="!apiKeys.length && !apiKeysLoading" depth="3">
              No API keys
            </NText>
            <NSpace v-else vertical :size="8">
              <div v-for="key in apiKeys" :key="key.id" class="key-row">
                <div class="key-row__top">
                  <NText strong :delete="!key.active">{{ key.name || '—' }}</NText>
                  <NSpace :size="4" :wrap-item="false">
                    <NTag v-if="key.mcp_only" type="info" size="tiny">MCP only</NTag>
                    <NTag v-if="!key.active" type="warning" size="tiny">off</NTag>
                  </NSpace>
                </div>
                <NText code style="font-size: 12px">{{ key.key_prefix }}…</NText>
                <NText depth="3" style="display: block; font-size: 12px">
                  Last used:
                  {{ key.last_used_at ? formatDate(key.last_used_at) : 'Never' }}
                </NText>
              </div>
            </NSpace>
          </NSpin>
        </template>
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.key-row__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
</style>
