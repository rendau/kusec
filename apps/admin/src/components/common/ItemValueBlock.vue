<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from 'vue'
import { NButton, NIcon, NSpace } from 'naive-ui'
import { Pencil } from '@vicons/tabler'

import ValueFormatChip from '@/components/common/ValueFormatChip.vue'
import { normalizeValueFormat } from '@/utils/format'

// CodeMirror is heavy — an instance per visible item froze the tab on
// "Expand all", so the editor is mounted only while a value is being edited;
// viewing is a plain <pre>.
const ValueEditor = defineAsyncComponent(
  () => import('@/components/common/ValueEditor.vue'),
)

const props = defineProps<{
  value: string
  /** Raw `value_format` of the item (drives the chip and the editor mode). */
  format?: string
  /** Persist the new value; a rejection keeps the editor open for a retry. */
  save: (value: string) => Promise<void>
}>()

const editorFormat = computed(() => normalizeValueFormat(props.format))

const editing = ref(false)
const saving = ref(false)
const draft = ref('')

function startEdit(): void {
  draft.value = props.value
  editing.value = true
}

function cancelEdit(): void {
  editing.value = false
}

async function submit(): Promise<void> {
  saving.value = true
  try {
    await props.save(draft.value)
    editing.value = false
  } catch {
    // The caller already surfaced the error — keep the draft for a retry.
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="value-block">
    <ValueFormatChip :format="format ?? ''" />

    <template v-if="!editing">
      <!-- Plain title instead of NTooltip: this block renders once per item,
           and popover components are what froze "Expand all" on big apps. -->
      <NButton
        class="value-block__edit"
        quaternary
        circle
        size="tiny"
        title="Edit value"
        aria-label="Edit value"
        @click="startEdit"
      >
        <template #icon>
          <NIcon :component="Pencil" />
        </template>
      </NButton>
      <pre class="value-block__pre" @dblclick="startEdit">{{ value }}</pre>
    </template>

    <template v-else>
      <ValueEditor
        v-model:value="draft"
        :format="editorFormat"
        min-height="0"
        max-height="320px"
      />
      <NSpace :size="8" justify="end" style="margin-top: 6px">
        <NButton size="tiny" :disabled="saving" @click="cancelEdit">
          Cancel
        </NButton>
        <NButton size="tiny" type="primary" :loading="saving" @click="submit">
          Save
        </NButton>
      </NSpace>
    </template>
  </div>
</template>

<style scoped>
.value-block {
  position: relative;
}

/* Sits in the top-right corner, to the left of the format chip. */
.value-block__edit {
  position: absolute;
  top: 2px;
  right: 52px;
  z-index: 1;
}

/* Mirrors the CodeMirror frame so view and edit modes look alike. */
.value-block__pre {
  margin: 0;
  padding: 5px 9px;
  min-height: 28px;
  max-height: 320px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.45;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  border: 1px solid rgba(128, 128, 128, 0.28);
  border-radius: 3px;
}
</style>
