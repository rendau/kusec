<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
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
import { createUser, updateUser } from '@/api/usr'
import type { UsrMain, UsrUpdateReq } from '@/api/types'

const props = defineProps<{
  show: boolean
  /** The user being edited, or `null` when creating a new one. */
  user: UsrMain | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const message = useMessage()

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

interface FormModel {
  name: string
  username: string
  password: string
  is_admin: boolean
  active: boolean
}

const model = reactive<FormModel>({
  name: '',
  username: '',
  password: '',
  is_admin: false,
  active: true,
})

const isEdit = computed(() => props.user !== null)

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
  username: [
    { required: true, message: 'Username is required', trigger: ['blur', 'input'] },
  ],
  // Password is mandatory only when creating a new user.
  password: isEdit.value
    ? []
    : [{ required: true, message: 'Password is required', trigger: ['blur', 'input'] }],
}))

// Reset the form whenever the modal opens, seeding it from the edited user.
watch(
  () => props.show,
  (show) => {
    if (!show) return
    const user = props.user
    model.name = user?.name ?? ''
    model.username = user?.username ?? ''
    model.password = ''
    model.is_admin = user?.is_admin ?? false
    model.active = user?.active ?? true
    formRef.value?.restoreValidation()
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
    if (props.user) {
      const update: UsrUpdateReq = {
        name: model.name,
        username: model.username,
        is_admin: model.is_admin,
        active: model.active,
      }
      // Only send the password when the admin actually typed a new one.
      if (model.password) update.password = model.password
      await updateUser(props.user.id, update)
      message.success('User updated')
    } else {
      await createUser({
        name: model.name,
        username: model.username,
        password: model.password,
        is_admin: model.is_admin,
        active: model.active,
      })
      message.success('User created')
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
    :title="isEdit ? 'Edit user' : 'New user'"
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
      <NFormItem label="Name" path="name">
        <NInput v-model:value="model.name" placeholder="Full name" clearable />
      </NFormItem>
      <NFormItem label="Username" path="username">
        <NInput v-model:value="model.username" placeholder="Login username" clearable />
      </NFormItem>
      <NFormItem
        :label="isEdit ? 'New password (leave blank to keep)' : 'Password'"
        path="password"
      >
        <NInput
          v-model:value="model.password"
          type="password"
          show-password-on="click"
          :placeholder="isEdit ? 'Unchanged' : 'Password'"
        />
      </NFormItem>
      <NFormItem label="Administrator" path="is_admin">
        <NSwitch v-model:value="model.is_admin" />
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
