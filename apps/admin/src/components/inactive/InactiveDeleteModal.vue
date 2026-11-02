<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NAlert, NButton, NInput, NModal, NScrollbar, NSpace, NTag, NText, useMessage } from 'naive-ui'

import { ApiError, apiErrorMessage } from '@/api/http'
import { deleteInactive } from '@/api/inactive'
import type { InactiveElement } from '@/api/types'
import { inactiveKindLabel } from '@/utils/inactive'

/**
 * Confirmation of deleting deactivated elements. One plain element takes a
 * click; a batch or an application (cascades to everything inside) must be
 * confirmed by typing, like the regular application delete.
 */
const CONFIRM_WORD = 'delete'

const props = defineProps<{
  show: boolean
  elements: InactiveElement[]
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  deleted: []
  /** Selection went stale on the server: the list has to be reloaded. */
  stale: []
}>()

const message = useMessage()

const confirmation = ref('')
const deleting = ref(false)

const hasApp = computed(() => props.elements.some((e) => e.kind === 'app'))
const needsTyping = computed(() => props.elements.length > 1 || hasApp.value)
const canDelete = computed(
  () => !needsTyping.value || confirmation.value.trim().toLowerCase() === CONFIRM_WORD,
)

const childCount = computed(() =>
  props.elements.reduce((sum, e) => sum + Number(e.child_count), 0),
)

watch(
  () => props.show,
  (show) => {
    if (show) confirmation.value = ''
  },
)

function close(): void {
  if (!deleting.value) emit('update:show', false)
}

async function confirmDelete(): Promise<void> {
  if (!canDelete.value) return

  deleting.value = true
  try {
    await deleteInactive({
      elements: props.elements.map((e) => ({ kind: e.kind, id: e.id })),
    })
    message.success(
      props.elements.length === 1 ? 'Element deleted' : `${props.elements.length} elements deleted`,
    )
    emit('update:show', false)
    emit('deleted')
  } catch (error) {
    if (error instanceof ApiError && error.code === 'element_active') {
      message.warning('Nothing deleted: an element was re-activated. The list has been refreshed.')
      emit('update:show', false)
      emit('stale')
    } else {
      message.error(apiErrorMessage(error, 'Failed to delete elements'))
    }
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="elements.length === 1 ? 'Delete element' : `Delete ${elements.length} elements`"
    style="max-width: 520px"
    :mask-closable="!deleting"
    @update:show="emit('update:show', $event)"
  >
    <NSpace vertical :size="12">
      <NAlert type="error" :show-icon="true" title="This cannot be undone">
        The selected elements are permanently deleted.
        <template v-if="childCount">
          Everything inside them goes too, even if it is active:
          {{ childCount }} nested element{{ childCount === 1 ? '' : 's' }}.
        </template>
      </NAlert>

      <NScrollbar style="max-height: 240px">
        <div v-for="e in elements" :key="`${e.kind}:${e.id}`" class="delete-row">
          <NTag size="tiny" :bordered="false">{{ inactiveKindLabel(e.kind) }}</NTag>
          <span class="delete-row__name">{{ e.name }}</span>
          <NText v-if="e.kind !== 'app'" depth="3" class="delete-row__where">
            {{ e.app_name }}<template v-if="e.parent_name"> / {{ e.parent_name }}</template>
          </NText>
        </div>
      </NScrollbar>

      <template v-if="needsTyping">
        <NText depth="3" style="font-size: 13px">
          Type <NText code>{{ CONFIRM_WORD }}</NText> to confirm:
        </NText>
        <NInput
          v-model:value="confirmation"
          :placeholder="CONFIRM_WORD"
          :disabled="deleting"
          @keyup.enter="confirmDelete"
        />
      </template>
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="deleting" @click="close">Cancel</NButton>
        <NButton type="error" :loading="deleting" :disabled="!canDelete" @click="confirmDelete">
          Delete
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.delete-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 3px 0;
  font-size: 13px;
}

.delete-row__name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  overflow-wrap: anywhere;
}

.delete-row__where {
  margin-left: auto;
  font-size: 12px;
  text-align: right;
}
</style>
