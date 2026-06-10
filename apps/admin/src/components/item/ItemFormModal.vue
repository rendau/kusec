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
import { createItem, updateItem } from '@/api/item'
import type { ItemMain, ItemUpdateReq } from '@/api/types'
import { useSecretOptions } from '@/composables/useSecretOptions'

const props = defineProps<{
  show: boolean
  /** The item being edited, or `null` when creating a new one. */
  item: ItemMain | null
  /** Pre-selected secret id when creating from within a secret context. */
  defaultSecretId?: string | null
  /** Lock the secret field (the item is bound to its secret). */
  lockSecret?: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const message = useMessage()
const {
  options: secretOptions,
  loading: secretsLoading,
  search,
  ensure,
} = useSecretOptions()

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

interface FormModel {
  secret_id: string | null
  key: string
  value: string
  description: string
  active: boolean
}

const model = reactive<FormModel>({
  secret_id: null,
  key: '',
  value: '',
  description: '',
  active: true,
})

const rules: FormRules = {
  secret_id: [
    {
      required: true,
      message: 'Secret is required',
      trigger: ['blur', 'change'],
      validator: (_rule, value: string | null) => value != null && value !== '',
    },
  ],
  key: [{ required: true, message: 'Key is required', trigger: ['blur', 'input'] }],
}

const isEdit = () => props.item !== null

onMounted(() => {
  void search()
})

// Reset the form whenever the modal opens, seeding it from the edited item.
watch(
  () => props.show,
  async (show) => {
    if (!show) return
    const item = props.item
    model.secret_id = item?.secret_id ?? props.defaultSecretId ?? null
    model.key = item?.key ?? ''
    model.value = item?.value ?? ''
    model.description = item?.description ?? ''
    model.active = item?.active ?? true
    formRef.value?.restoreValidation()
    if (model.secret_id) await ensure(model.secret_id)
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
    if (props.item) {
      const update: ItemUpdateReq = {
        key: model.key,
        value: model.value,
        description: model.description,
        active: model.active,
      }
      if (model.secret_id) update.secret_id = model.secret_id
      await updateItem(props.item.id, update)
      message.success('Item updated')
    } else {
      await createItem({
        secret_id: model.secret_id as string,
        key: model.key,
        value: model.value,
        description: model.description,
        active: model.active,
      })
      message.success('Item created')
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
    :title="isEdit() ? 'Edit item' : 'New item'"
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
      <NFormItem label="Secret" path="secret_id">
        <NSelect
          v-model:value="model.secret_id"
          :options="secretOptions"
          :loading="secretsLoading"
          :disabled="lockSecret"
          :clearable="!lockSecret"
          filterable
          remote
          placeholder="Select a secret"
          @search="search"
        />
      </NFormItem>
      <NFormItem label="Key" path="key">
        <NInput v-model:value="model.key" placeholder="e.g. password" clearable />
      </NFormItem>
      <NFormItem label="Value" path="value">
        <NInput
          v-model:value="model.value"
          type="textarea"
          placeholder="Secret value"
          :autosize="{ minRows: 1, maxRows: 6 }"
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
