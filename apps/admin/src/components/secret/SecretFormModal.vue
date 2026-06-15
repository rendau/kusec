<script setup lang="ts">
import { onMounted, reactive } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
} from 'naive-ui'
import type { FormRules } from 'naive-ui'

import { createSecret, updateSecret } from '@/api/secret'
import type { SecretMain, SecretUpdateReq } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useEntityForm } from '@/composables/useEntityForm'

const props = defineProps<{
  show: boolean
  /** The secret being edited, or `null` when creating a new one. */
  secret: SecretMain | null
  /** Pre-selected app id when creating from within an app context. */
  defaultAppId?: string | null
  /** Pre-filled slug for a new secret (e.g. "main" for the app's first secret). */
  defaultSlug?: string | null
  /** Lock the application field (the secret is bound to its app). */
  lockApp?: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const { options: appOptions, loading: appsLoading, search, ensure } = useAppOptions()

interface FormModel {
  app_id: string | null
  slug_name: string
  description: string
  active: boolean
  /** K8s secret type; null/empty = Opaque (backend default). */
  kube_type: string | null
}

const model = reactive<FormModel>({
  app_id: null,
  slug_name: '',
  description: '',
  active: true,
  kube_type: null,
})

/** Well-known k8s secret types; custom values can be typed in (tag mode). */
const kubeTypeOptions = [
  'kubernetes.io/basic-auth',
  'kubernetes.io/tls',
  'kubernetes.io/dockerconfigjson',
  'kubernetes.io/ssh-auth',
].map((value) => ({ label: value, value }))

const rules: FormRules = {
  app_id: [
    {
      required: true,
      message: 'Application is required',
      trigger: ['blur', 'change'],
      validator: (_rule, value: string | null) => value != null && value !== '',
    },
  ],
  slug_name: [
    { required: true, message: 'Slug is required', trigger: ['blur', 'input'] },
  ],
}

const { formRef, submitting, isEdit, submit } = useEntityForm<SecretMain>({
  show: () => props.show,
  entity: () => props.secret,
  seed: async (secret) => {
    model.app_id = secret?.app_id ?? props.defaultAppId ?? null
    // Имя нового секрета: defaultSlug (напр. "main" для первого секрета app).
    model.slug_name = secret?.slug_name ?? props.defaultSlug ?? ''
    model.description = secret?.description ?? ''
    model.active = secret?.active ?? true
    model.kube_type = secret?.kube_type || null
    // Make sure the selected app is present in the options list.
    if (model.app_id) await ensure(model.app_id)
  },
  create: () =>
    createSecret({
      app_id: model.app_id as string,
      slug_name: model.slug_name,
      description: model.description,
      active: model.active,
      kube_type: model.kube_type ?? '',
    }),
  update: (secret) => {
    const update: SecretUpdateReq = {
      slug_name: model.slug_name,
      description: model.description,
      active: model.active,
      kube_type: model.kube_type ?? '',
    }
    if (model.app_id) update.app_id = model.app_id
    return updateSecret(secret.id, update)
  },
  messages: { created: 'Secret created', updated: 'Secret updated' },
  onSaved: () => {
    emit('saved')
    close()
  },
})

onMounted(() => {
  void search()
})

function close(): void {
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit ? 'Edit secret' : 'New secret'"
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
          :disabled="lockApp"
          :clearable="!lockApp"
          filterable
          remote
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
      <NFormItem label="K8s type" path="kube_type">
        <NSelect
          v-model:value="model.kube_type"
          :options="kubeTypeOptions"
          tag
          filterable
          clearable
          placeholder="Opaque (default)"
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
          {{ isEdit ? 'Save' : 'Create' }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
