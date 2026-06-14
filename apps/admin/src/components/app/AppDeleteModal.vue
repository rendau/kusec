<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NInput,
  NModal,
  NSpace,
  NText,
  useMessage,
} from 'naive-ui'

import { apiErrorMessage } from '@/api/http'
import { deleteApp } from '@/api/app'
import type { AppMain } from '@/api/types'

/**
 * Type-to-confirm deletion of an application. Deleting an app cascades to
 * all of its secrets, config maps and their items, so a plain "OK" dialog is
 * too easy to misclick — the user must type the application name first.
 */
const props = defineProps<{
  show: boolean
  app: AppMain | null
  /** Number of secrets that will be deleted along with the app. */
  secretCount?: number
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  deleted: []
}>()

const message = useMessage()

const confirmation = ref('')
const deleting = ref(false)

const nameMatches = computed(
  () => props.app !== null && confirmation.value.trim() === props.app.name,
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
  const app = props.app
  if (!app || !nameMatches.value) return

  deleting.value = true
  try {
    await deleteApp(app.id)
    message.success('Application deleted')
    emit('update:show', false)
    emit('deleted')
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to delete application'))
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Delete application"
    style="max-width: 480px"
    :mask-closable="!deleting"
    @update:show="emit('update:show', $event)"
  >
    <NSpace vertical :size="12">
      <NAlert type="error" :show-icon="true" title="This cannot be undone">
        Deleting
        <NText strong>{{ app?.name }}</NText>
        also permanently deletes
        <template v-if="secretCount">
          its {{ secretCount }} secret{{ secretCount === 1 ? '' : 's' }}, all of
          their items, and every config map and config item.
        </template>
        <template v-else>
          all of its secrets, config maps and their items.
        </template>
      </NAlert>

      <NText depth="3" style="font-size: 13px">
        Type <NText code>{{ app?.name }}</NText> to confirm:
      </NText>
      <NInput
        v-model:value="confirmation"
        :placeholder="app?.name ?? ''"
        :disabled="deleting"
        @keyup.enter="confirmDelete"
      />
    </NSpace>

    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="deleting" @click="close">Cancel</NButton>
        <NButton
          type="error"
          :loading="deleting"
          :disabled="!nameMatches"
          @click="confirmDelete"
        >
          Delete application
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
