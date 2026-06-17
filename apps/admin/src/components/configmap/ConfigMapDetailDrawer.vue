<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NDrawer,
  NDrawerContent,
  NSpin,
  NTag,
  NText,
  NTooltip,
} from 'naive-ui'

import { getConfigMap } from '@/api/configmap'
import { listConfigItems } from '@/api/configitem'
import { getClusterConfigMap } from '@/api/kube'
import type { ConfigItemMain } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useClipboard } from '@/composables/useClipboard'
import { useDrawerResource } from '@/composables/useDrawerResource'
import { formatDate } from '@/utils/format'
import type { ExportPair } from '@/utils/export'

import ClusterResourcePanel from '@/components/kube/ClusterResourcePanel.vue'
import ResourceExportPanel from '@/components/kube/ResourceExportPanel.vue'

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

// Config items power the key/value export; loaded alongside the details.
const items = ref<ConfigItemMain[]>([])

watch(
  [() => props.show, () => props.configMapId],
  async ([show, id]) => {
    if (!show || !id) {
      items.value = []
      return
    }
    try {
      const rep = await listConfigItems({ configmap_id: id })
      items.value = rep.results ?? []
    } catch {
      items.value = []
    }
  },
  { immediate: true },
)

const exportPairs = computed<ExportPair[]>(() =>
  items.value.map((item) => ({ key: item.key, value: item.value })),
)

const exportBaseName = computed(
  () =>
    configMap.value?.kube_configmap_name || configMap.value?.slug_name || 'configmap',
)

function loadCluster() {
  return getClusterConfigMap(props.configMapId!)
}
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
                  @click="
                    copy(configMap!.kube_configmap_name, 'K8s configmap name copied')
                  "
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

      <template v-if="configMap">
        <NDivider style="margin: 16px 0 12px" />
        <ResourceExportPanel :pairs="exportPairs" :base-name="exportBaseName" />
        <NDivider style="margin: 16px 0 12px" />
        <ClusterResourcePanel :loader="loadCluster" kind="config map" />
      </template>
    </NDrawerContent>
  </NDrawer>
</template>
