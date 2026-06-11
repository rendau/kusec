<script setup lang="ts">
import { reactive } from 'vue'
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

import { createApp, updateApp } from '@/api/app'
import type { AppCreateRep, AppMain } from '@/api/types'
import { useEntityForm } from '@/composables/useEntityForm'
import { useKubeNamespaces } from '@/composables/useKubeNamespaces'

const props = defineProps<{
  show: boolean
  /** The app being edited, or `null` when creating a new one. */
  app: AppMain | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  /** id присутствует только при создании (для перехода на новый app). */
  saved: [createdId?: string]
}>()

const {
  options: namespaceOptions,
  loading: namespacesLoading,
  ensure: ensureNamespaces,
} = useKubeNamespaces()

interface FormModel {
  namespace: string
  name: string
  slug_name: string
  description: string
  active: boolean
}

const model = reactive<FormModel>({
  namespace: '',
  name: '',
  slug_name: '',
  description: '',
  active: true,
})

const rules: FormRules = {
  namespace: [
    { required: true, message: 'Namespace is required', trigger: ['blur', 'input'] },
  ],
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
  slug_name: [
    { required: true, message: 'Slug is required', trigger: ['blur', 'input'] },
  ],
}

const { formRef, submitting, isEdit, submit } = useEntityForm<AppMain, AppCreateRep>({
  show: () => props.show,
  entity: () => props.app,
  seed: (app) => {
    // "default" предвыбран и в кластере (он там есть), и вне кластера.
    model.namespace = app?.namespace ?? 'default'
    model.name = app?.name ?? ''
    model.slug_name = app?.slug_name ?? ''
    model.description = app?.description ?? ''
    model.active = app?.active ?? true
    // Подтянуть namespaces кластера для выпадашки (кэшируется на сессию).
    void ensureNamespaces()
  },
  create: () =>
    createApp({
      namespace: model.namespace,
      name: model.name,
      slug_name: model.slug_name,
      description: model.description,
      active: model.active,
    }),
  update: (app) =>
    updateApp(app.id, {
      namespace: model.namespace,
      name: model.name,
      slug_name: model.slug_name,
      description: model.description,
      active: model.active,
    }),
  messages: { created: 'Application created', updated: 'Application updated' },
  onSaved: (created) => {
    emit('saved', created?.id)
    close()
  },
})

function close(): void {
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit ? 'Edit application' : 'New application'"
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
        <!-- tag: можно выбрать существующий namespace кластера или ввести
             новый (вне кластера список пуст — обычный свободный ввод). -->
        <NSelect
          v-model:value="model.namespace"
          :options="namespaceOptions"
          :loading="namespacesLoading"
          filterable
          tag
          placeholder="e.g. payments"
        />
      </NFormItem>
      <NFormItem label="Name" path="name">
        <NInput
          v-model:value="model.name"
          placeholder="Application name"
          clearable
        />
      </NFormItem>
      <NFormItem label="Slug" path="slug_name">
        <NInput
          v-model:value="model.slug_name"
          placeholder="e.g. payments-api"
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
          {{ isEdit ? 'Save' : 'Create' }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
