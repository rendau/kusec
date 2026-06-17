<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  NButton,
  NFlex,
  NIcon,
  NRadioButton,
  NRadioGroup,
  NText,
  useMessage,
} from 'naive-ui'
import { Copy, Download } from '@vicons/tabler'

import { useClipboard } from '@/composables/useClipboard'
import {
  EXPORT_FORMATS,
  downloadText,
  formatKeyValues,
  type ExportFormat,
  type ExportPair,
} from '@/utils/export'

const props = defineProps<{
  /** Flat key/value pairs to export (binary values stay base64). */
  pairs: ExportPair[]
  /** Download file base name (without extension). */
  baseName: string
}>()

const message = useMessage()
const { copy } = useClipboard()

const format = ref<ExportFormat>('env')

const formats = Object.entries(EXPORT_FORMATS).map(([value, meta]) => ({
  value: value as ExportFormat,
  label: meta.label,
}))

const text = computed(() => formatKeyValues(props.pairs, format.value))

function onCopy(): void {
  if (!props.pairs.length) {
    message.warning('Nothing to export')
    return
  }
  void copy(text.value, `Copied as ${EXPORT_FORMATS[format.value].label}`)
}

function onDownload(): void {
  if (!props.pairs.length) {
    message.warning('Nothing to export')
    return
  }
  downloadText(text.value, props.baseName || 'export', format.value)
}
</script>

<template>
  <div class="export">
    <NText depth="3" class="export__title">Export ({{ pairs.length }} keys)</NText>
    <NFlex align="center" :size="8" :wrap="false">
      <NRadioGroup v-model:value="format" size="small">
        <NRadioButton
          v-for="f in formats"
          :key="f.value"
          :value="f.value"
          :label="f.label"
        />
      </NRadioGroup>
      <span style="flex: 1" />
      <NButton size="small" :disabled="!pairs.length" @click="onCopy">
        <template #icon>
          <NIcon :component="Copy" />
        </template>
        Copy
      </NButton>
      <NButton size="small" :disabled="!pairs.length" @click="onDownload">
        <template #icon>
          <NIcon :component="Download" />
        </template>
        Download
      </NButton>
    </NFlex>
  </div>
</template>

<style scoped>
.export {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.export__title {
  font-size: 12px;
}
</style>
