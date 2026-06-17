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

import { getClusterSecret } from '@/api/kube'
import { listItems } from '@/api/item'
import { getSecret } from '@/api/secret'
import type { ItemMain } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useClipboard } from '@/composables/useClipboard'
import { useDrawerResource } from '@/composables/useDrawerResource'
import { formatDate } from '@/utils/format'
import type { ExportPair } from '@/utils/export'

import ClusterResourcePanel from '@/components/kube/ClusterResourcePanel.vue'
import ResourceExportPanel from '@/components/kube/ResourceExportPanel.vue'

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

// Items power the key/value export; loaded alongside the secret details.
const items = ref<ItemMain[]>([])

watch(
  [() => props.show, () => props.secretId],
  async ([show, id]) => {
    if (!show || !id) {
      items.value = []
      return
    }
    try {
      const rep = await listItems({ secret_id: id })
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
  () => secret.value?.kube_secret_name || secret.value?.slug_name || 'secret',
)

function loadCluster() {
  return getClusterSecret(props.secretId!)
}
</script>

<template>
  <NDrawer
    :show="show"
    width="min(420px, 100vw)"
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

      <template v-if="secret">
        <NDivider style="margin: 16px 0 12px" />
        <ResourceExportPanel :pairs="exportPairs" :base-name="exportBaseName" />
        <NDivider style="margin: 16px 0 12px" />
        <ClusterResourcePanel :loader="loadCluster" kind="secret" />
      </template>
    </NDrawerContent>
  </NDrawer>
</template>
