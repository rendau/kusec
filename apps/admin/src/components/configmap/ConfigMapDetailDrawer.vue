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

import { getConfigMap } from '@/api/configmap'
import { useAppOptions } from '@/composables/useAppOptions'
import { useClipboard } from '@/composables/useClipboard'
import { useDrawerResource } from '@/composables/useDrawerResource'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  show: boolean
  /** Id of the config map to display; details are (re)fetched when it changes. */
  configMapId: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { nameOf, ensure } = useAppOptions()
const { copy } = useClipboard()

const { loading, item: configMap } = useDrawerResource({
  show: () => props.show,
  id: () => props.configMapId,
  fetch: getConfigMap,
  onLoaded: (loaded) => ensure(loaded.app_id),
  onError: () => emit('update:show', false),
})
</script>

<template>
  <NDrawer
    :show="show"
    width="min(420px, 100vw)"
    placement="right"
    @update:show="emit('update:show', $event)"
  >
    <NDrawerContent title="Config map details" closable>
      <NSpin :show="loading">
        <NDescriptions
          v-if="configMap"
          :column="1"
          label-placement="top"
          bordered
          size="small"
        >
          <NDescriptionsItem label="ID">
            <NText code>{{ configMap.id }}</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="Slug">
            {{ configMap.slug_name }}
          </NDescriptionsItem>
          <NDescriptionsItem label="K8s configmap">
            <NTooltip v-if="configMap.kube_configmap_name">
              <template #trigger>
                <NText
                  code
                  style="cursor: pointer"
                  @click="copy(configMap!.kube_configmap_name, 'K8s configmap name copied')"
                >
                  {{ configMap.kube_configmap_name }}
                </NText>
              </template>
              Copy
            </NTooltip>
            <NText v-else depth="3">—</NText>
          </NDescriptionsItem>
          <NDescriptionsItem label="Application">
            <NTag size="small" type="info">{{ nameOf(configMap.app_id) }}</NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <NTag :type="configMap.active ? 'success' : 'default'" size="small">
              {{ configMap.active ? 'Active' : 'Inactive' }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Description">
            {{ configMap.description || '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Created">
            {{ formatDate(configMap.created_at) }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Updated">
            {{ formatDate(configMap.updated_at) }}
          </NDescriptionsItem>
        </NDescriptions>
        <div v-else style="min-height: 120px" />
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>
