<script setup lang="ts">
import {
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NSpin,
  NTag,
  NText,
  NTooltip,
} from 'naive-ui'

import { getSecret } from '@/api/secret'
import { useAppOptions } from '@/composables/useAppOptions'
import { useClipboard } from '@/composables/useClipboard'
import { useDrawerResource } from '@/composables/useDrawerResource'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  show: boolean
  /** Id of the secret to display; details are (re)fetched when it changes. */
  secretId: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { nameOf, ensure } = useAppOptions()
const { copy } = useClipboard()

const { loading, item: secret } = useDrawerResource({
  show: () => props.show,
  id: () => props.secretId,
  fetch: getSecret,
  onLoaded: (loaded) => ensure(loaded.app_id),
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
          <NDescriptionsItem label="K8s secret">
            <NTooltip v-if="secret.kube_secret_name">
              <template #trigger>
                <NText
                  code
                  style="cursor: pointer"
                  @click="copy(secret!.kube_secret_name, 'K8s secret name copied')"
                >
                  {{ secret.kube_secret_name }}
                </NText>
              </template>
              Copy
            </NTooltip>
            <NText v-else depth="3">—</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="K8s type">
            <NText code>{{ secret.kube_type || 'Opaque' }}</NText>
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
