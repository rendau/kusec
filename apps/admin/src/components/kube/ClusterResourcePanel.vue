<script setup lang="ts">
import { ref } from 'vue'
import { NAlert, NButton, NFlex, NIcon, NSpin, NTag, NText, useMessage } from 'naive-ui'
import { Cloud, Copy, Refresh } from '@vicons/tabler'

import { apiErrorMessage } from '@/api/http'
import type { KubeClusterResourceRep } from '@/api/types'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{
  /** Loads the live cluster object (secret or config map). */
  loader: () => Promise<KubeClusterResourceRep>
  /** Object kind — only affects empty-state wording. */
  kind: 'secret' | 'config map'
}>()

const message = useMessage()
const { copy } = useClipboard()

const loading = ref(false)
const loaded = ref(false)
const data = ref<KubeClusterResourceRep | null>(null)

async function load(): Promise<void> {
  loading.value = true
  try {
    data.value = await props.loader()
    loaded.value = true
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to read from cluster'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="cluster">
    <NFlex align="center" justify="space-between">
      <NText depth="3" class="cluster__title">Live in cluster</NText>
      <NButton size="small" :loading="loading" @click="load">
        <template #icon>
          <NIcon :component="loaded ? Refresh : Cloud" />
        </template>
        {{ loaded ? 'Reload' : 'Show from cluster' }}
      </NButton>
    </NFlex>

    <NSpin :show="loading">
      <div v-if="loaded && data" class="cluster__body">
        <NAlert
          v-if="!data.in_cluster"
          type="info"
          :bordered="false"
          :show-icon="false"
        >
          Backend runs outside a cluster — live state is unavailable.
        </NAlert>

        <NAlert
          v-else-if="!data.found"
          type="warning"
          :bordered="false"
          :show-icon="false"
        >
          This {{ kind }} is not in the cluster yet (not synced).
        </NAlert>

        <template v-else>
          <NFlex align="center" :size="6" :wrap="true" class="cluster__meta">
            <NTag size="small" type="info">{{ data.namespace }}/{{ data.name }}</NTag>
            <NTag v-if="data.type" size="small">{{ data.type }}</NTag>
            <NTag size="small" :type="data.managed ? 'success' : 'default'">
              {{ data.managed ? 'managed by kusec' : 'unmanaged' }}
            </NTag>
          </NFlex>

          <div v-if="!data.items.length" class="cluster__empty">
            <NText depth="3">No keys</NText>
          </div>

          <div v-else class="cluster__items">
            <div v-for="item in data.items" :key="item.key" class="cluster__item">
              <div class="cluster__item-head">
                <NText code class="cluster__key">{{ item.key }}</NText>
                <NTag v-if="item.encoding === 'base64'" size="tiny" :bordered="false">
                  base64
                </NTag>
                <span style="flex: 1" />
                <NButton
                  quaternary
                  circle
                  size="tiny"
                  aria-label="Copy value"
                  @click="copy(item.value)"
                >
                  <template #icon>
                    <NIcon :component="Copy" />
                  </template>
                </NButton>
              </div>
              <pre class="cluster__value">{{ item.value }}</pre>
            </div>
          </div>
        </template>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.cluster {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cluster__title {
  font-size: 12px;
}

.cluster__body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cluster__meta {
  margin-top: 2px;
}

.cluster__items {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cluster__item {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 4px;
  padding: 6px 8px;
}

.cluster__item-head {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cluster__key {
  font-size: 12.5px;
  word-break: break-all;
}

.cluster__value {
  margin: 6px 0 0;
  padding: 6px 8px;
  background: rgba(128, 128, 128, 0.1);
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow: auto;
}

.cluster__empty {
  padding: 8px 0;
}
</style>
