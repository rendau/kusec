<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NInput,
  NInputGroup,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NText,
} from 'naive-ui'
import type { FormRules, SelectOption } from 'naive-ui'

import { createApiKey, updateApiKey } from '@/api/apikey'
import { listUsers } from '@/api/usr'
import type { ApiKeyCreateRep, ApiKeyMain, ApiKeyUpdateReq } from '@/api/types'
import { useClipboard } from '@/composables/useClipboard'
import { useEntityForm } from '@/composables/useEntityForm'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  show: boolean
  /** The key being edited, or `null` when issuing a new one. */
  apiKey: ApiKeyMain | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const authStore = useAuthStore()
const { copy } = useClipboard()

interface FormModel {
  name: string
  mcp_only: boolean
  active: boolean
  /** Owner user id as a string (admin-only); `null` = the current user. */
  usr_id: string | null
}

const model = reactive<FormModel>({
  name: '',
  mcp_only: true,
  active: true,
  usr_id: null,
})

/**
 * The created key value, revealed exactly once by the backend. While set, the
 * modal shows the copy screen instead of the form.
 */
const createdKey = ref('')

const { formRef, submitting, isEdit, submit } = useEntityForm<
  ApiKeyMain,
  ApiKeyCreateRep
>({
  show: () => props.show,
  entity: () => props.apiKey,
  seed: (apiKey) => {
    model.name = apiKey?.name ?? ''
    model.mcp_only = apiKey?.mcp_only ?? true
    model.active = apiKey?.active ?? true
    model.usr_id = null
    createdKey.value = ''
    if (!isEdit.value && authStore.isAdmin) void searchUsers('')
  },
  create: () =>
    createApiKey({
      name: model.name,
      mcp_only: model.mcp_only,
      ...(model.usr_id ? { usr_id: model.usr_id } : {}),
    }),
  update: (apiKey) => {
    const update: ApiKeyUpdateReq = {
      name: model.name,
      mcp_only: model.mcp_only,
      active: model.active,
    }
    return updateApiKey(apiKey.id, update)
  },
  messages: { created: 'API key created', updated: 'API key updated' },
  onSaved: (created) => {
    emit('saved')
    // On create keep the modal open to reveal the key (the only chance).
    if (created?.key) {
      createdKey.value = created.key
    } else {
      close()
    }
  },
})

// ── Owner select (admins may issue keys for other users) ───

const usrOptions = ref<SelectOption[]>([])
const usrLoading = ref(false)

async function searchUsers(query: string): Promise<void> {
  usrLoading.value = true
  try {
    const rep = await listUsers({
      list_params: { page: 0, page_size: 50 },
      ...(query.trim() ? { search: query.trim() } : {}),
    })
    usrOptions.value = (rep.results ?? []).map((u) => ({
      label: u.name ? `${u.name} (${u.username})` : u.username,
      value: String(u.id),
    }))
  } catch {
    // список владельцев — вспомогательный; ошибку не эскалируем
  } finally {
    usrLoading.value = false
  }
}

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
}))

// Reset the reveal screen whenever the modal is closed.
watch(
  () => props.show,
  (show) => {
    if (!show) createdKey.value = ''
  },
)

function close(): void {
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="createdKey ? 'API key created' : isEdit ? 'Edit API key' : 'New API key'"
    style="max-width: 520px"
    :mask-closable="!submitting && !createdKey"
    @update:show="emit('update:show', $event)"
  >
    <!-- Reveal screen: the key value exists only in this response. -->
    <template v-if="createdKey">
      <NSpace vertical :size="12">
        <NAlert type="warning" title="Copy the key now">
          It is shown only once — kusec stores just a hash and cannot display it
          again.
        </NAlert>
        <NInputGroup>
          <NInput
            :value="createdKey"
            readonly
            style="font-family: var(--n-font-family-mono, monospace)"
          />
          <NButton type="primary" @click="copy(createdKey, 'API key copied')">
            Copy
          </NButton>
        </NInputGroup>
        <NText depth="3" style="font-size: 12px">
          Pass it to the agent as
          <code>Authorization: Bearer &lt;key&gt;</code> on the MCP endpoint.
        </NText>
      </NSpace>
    </template>

    <NForm
      v-else
      ref="formRef"
      :model="model"
      :rules="rules"
      label-placement="top"
      :disabled="submitting"
    >
      <NFormItem label="Name" path="name">
        <NInput
          v-model:value="model.name"
          placeholder="What is this key for (e.g. mcp agent laptop)"
          clearable
        />
      </NFormItem>
      <NFormItem v-if="!isEdit && authStore.isAdmin" label="Owner" path="usr_id">
        <NSelect
          v-model:value="model.usr_id"
          :options="usrOptions"
          :loading="usrLoading"
          remote
          filterable
          clearable
          placeholder="Yourself"
          @search="searchUsers"
        />
      </NFormItem>
      <NFormItem label="MCP only" path="mcp_only">
        <NSpace vertical :size="4">
          <NSwitch v-model:value="model.mcp_only" />
          <NText depth="3" style="font-size: 12px">
            The key is accepted only by the MCP endpoint; the main API rejects
            it. Recommended for AI agents — keeps secret values unreadable even
            with the key.
          </NText>
        </NSpace>
      </NFormItem>
      <NFormItem v-if="isEdit" label="Active" path="active">
        <NSwitch v-model:value="model.active" />
      </NFormItem>
    </NForm>

    <template #footer>
      <NSpace justify="end">
        <template v-if="createdKey">
          <NButton type="primary" @click="close">Done</NButton>
        </template>
        <template v-else>
          <NButton :disabled="submitting" @click="close">Cancel</NButton>
          <NButton type="primary" :loading="submitting" @click="submit">
            {{ isEdit ? 'Save' : 'Create' }}
          </NButton>
        </template>
      </NSpace>
    </template>
  </NModal>
</template>
