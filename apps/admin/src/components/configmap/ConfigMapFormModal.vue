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

import { createConfigMap, updateConfigMap } from '@/api/configmap'
import type { ConfigMapMain, ConfigMapUpdateReq } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useEntityForm } from '@/composables/useEntityForm'

const props = defineProps<{
  show: boolean
  /** The config map being edited, or `null` when creating a new one. */
  configMap: ConfigMapMain | null
  /** Pre-selected app id when creating from within an app context. */
  defaultAppId?: string | null
  /** Lock the application field (the config map is bound to its app). */
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
      validator: (_rule, value: string | null) => value != null && value !== '',
    },
  ],
  slug_name: [
    { required: true, message: 'Slug is required', trigger: ['blur', 'input'] },
  ],
}

const { formRef, submitting, isEdit, submit } = useEntityForm<ConfigMapMain>({
  show: () => props.show,
  entity: () => props.configMap,
  seed: async (configMap) => {
    model.app_id = configMap?.app_id ?? props.defaultAppId ?? null
    model.slug_name = configMap?.slug_name ?? ''
    model.description = configMap?.description ?? ''
    model.active = configMap?.active ?? true
    // Make sure the selected app is present in the options list.
    if (model.app_id) await ensure(model.app_id)
  },
  create: () =>
    createConfigMap({
      app_id: model.app_id as string,
      slug_name: model.slug_name,
      description: model.description,
      active: model.active,
    }),
  update: (configMap) => {
    const update: ConfigMapUpdateReq = {
      slug_name: model.slug_name,
      description: model.description,
      active: model.active,
    }
    if (model.app_id) update.app_id = model.app_id
    return updateConfigMap(configMap.id, update)
  },
  messages: { created: 'Config map created', updated: 'Config map updated' },
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
    :title="isEdit ? 'Edit config map' : 'New config map'"
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
          placeholder="e.g. app-config"
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
