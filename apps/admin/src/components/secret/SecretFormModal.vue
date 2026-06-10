<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ApiError } from '@/api/http'
import { createSecret, updateSecret } from '@/api/secret'
import type { SecretMain, SecretUpdateReq } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'

const props = defineProps<{
  show: boolean
  /** The secret being edited, or `null` when creating a new one. */
  secret: SecretMain | null
  /** Pre-selected app id when creating from within an app context. */
  defaultAppId?: string | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const message = useMessage()
const { options: appOptions, loading: appsLoading, search, ensure } = useAppOptions()

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

interface FormModel {
  app_id: string | null
  slug_name: string
  description: string
  active: boolean
}

const model = reactive<FormModel>({
  app_id: null,
  slug_name: '',
  description: '',
  active: true,
})

const rules: FormRules = {
  app_id: [
    {
      required: true,
      message: 'Application is required',
      trigger: ['blur', 'change'],
      validator: (_rule, value: string | null) =>
        value != null && value !== '',
    },
  ],
  slug_name: [
    { required: true, message: 'Slug is required', trigger: ['blur', 'input'] },
  ],
}

const isEdit = () => props.secret !== null

onMounted(() => {
  void search()
})

// Reset the form whenever the modal opens, seeding it from the edited secret.
watch(
  () => props.show,
  async (show) => {
    if (!show) return
    const secret = props.secret
    model.app_id = secret?.app_id ?? props.defaultAppId ?? null
    model.slug_name = secret?.slug_name ?? ''
    model.description = secret?.description ?? ''
    model.active = secret?.active ?? true
    formRef.value?.restoreValidation()
    // Make sure the selected app is present in the options list.
    if (model.app_id) await ensure(model.app_id)
  },
)

function close(): void {
  emit('update:show', false)
}

async function submit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  submitting.value = true
  try {
    if (props.secret) {
      const update: SecretUpdateReq = {
        slug_name: model.slug_name,
        description: model.description,
        active: model.active,
      }
      if (model.app_id) update.app_id = model.app_id
      await updateSecret(props.secret.id, update)
      message.success('Secret updated')
    } else {
      await createSecret({
        app_id: model.app_id as string,
        slug_name: model.slug_name,
        description: model.description,
        active: model.active,
      })
      message.success('Secret created')
    }
    emit('saved')
    close()
  } catch (error) {
    message.error(
      error instanceof ApiError ? error.message : 'Unexpected error, please try again',
    )
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit() ? 'Edit secret' : 'New secret'"
    style="max-width: 520px"
    :mask-closable="!submitting"
    @update:show="emit('update:show', $event)"
  >
    <NForm
      ref="formRef"
      :model="model"
      :rules="rules"
      label-placement="top"
      :disabled="submitting"
    >
      <NFormItem label="Application" path="app_id">
        <NSelect
          v-model:value="model.app_id"
          :options="appOptions"
          :loading="appsLoading"
          filterable
          remote
          clearable
          placeholder="Select an application"
          @search="search"
        />
      </NFormItem>
      <NFormItem label="Slug" path="slug_name">
        <NInput
          v-model:value="model.slug_name"
          placeholder="e.g. db-credentials"
          clearable
        />
      </NFormItem>
      <NFormItem label="Description" path="description">
        <NInput
          v-model:value="model.description"
          type="textarea"
          placeholder="Optional description"
          :autosize="{ minRows: 2, maxRows: 5 }"
        />
      </NFormItem>
      <NFormItem label="Active" path="active">
        <NSwitch v-model:value="model.active" />
      </NFormItem>
    </NForm>

    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton type="primary" :loading="submitting" @click="submit">
          {{ isEdit() ? 'Save' : 'Create' }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
