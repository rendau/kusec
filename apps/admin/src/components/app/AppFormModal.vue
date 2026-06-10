<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSpace,
  NSwitch,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ApiError } from '@/api/http'
import { createApp, updateApp } from '@/api/app'
import type { AppMain } from '@/api/types'

const props = defineProps<{
  show: boolean
  /** The app being edited, or `null` when creating a new one. */
  app: AppMain | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const message = useMessage()

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

interface FormModel {
  namespace: string
  name: string
  description: string
  active: boolean
}

const model = reactive<FormModel>({
  namespace: '',
  name: '',
  description: '',
  active: true,
})

const rules: FormRules = {
  namespace: [
    { required: true, message: 'Namespace is required', trigger: ['blur', 'input'] },
  ],
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
}

const isEdit = () => props.app !== null

// Reset the form whenever the modal opens, seeding it from the edited app.
watch(
  () => props.show,
  (show) => {
    if (!show) return
    const app = props.app
    model.namespace = app?.namespace ?? ''
    model.name = app?.name ?? ''
    model.description = app?.description ?? ''
    model.active = app?.active ?? true
    formRef.value?.restoreValidation()
  },
)

function close(): void {
  emit('update:show', false)
}

async function applyFieldErrors(error: ApiError): Promise<void> {
  message.error(error.message)
}

async function submit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  submitting.value = true
  try {
    if (props.app) {
      await updateApp(props.app.id, {
        namespace: model.namespace,
        name: model.name,
        description: model.description,
        active: model.active,
      })
      message.success('Application updated')
    } else {
      await createApp({
        namespace: model.namespace,
        name: model.name,
        description: model.description,
        active: model.active,
      })
      message.success('Application created')
    }
    emit('saved')
    close()
  } catch (error) {
    if (error instanceof ApiError) {
      await applyFieldErrors(error)
    } else {
      message.error('Unexpected error, please try again')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit() ? 'Edit application' : 'New application'"
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
      <NFormItem label="Namespace" path="namespace">
        <NInput
          v-model:value="model.namespace"
          placeholder="e.g. payments"
          clearable
        />
      </NFormItem>
      <NFormItem label="Name" path="name">
        <NInput
          v-model:value="model.name"
          placeholder="Application name"
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
