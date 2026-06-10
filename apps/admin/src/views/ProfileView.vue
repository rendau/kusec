<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NInput,
  NSpace,
  NTag,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormItemRule, FormRules } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { ApiError } from '@/api/http'
import type { UsrUpdateProfileReq } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const message = useMessage()
const authStore = useAuthStore()
const { profile } = storeToRefs(authStore)

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

interface FormModel {
  name: string
  username: string
  password: string
  passwordConfirm: string
}

const model = reactive<FormModel>({
  name: '',
  username: '',
  password: '',
  passwordConfirm: '',
})

// Seed (and re-seed) the form from the loaded profile.
watch(
  profile,
  (value) => {
    model.name = value?.name ?? ''
    model.username = value?.username ?? ''
    model.password = ''
    model.passwordConfirm = ''
    formRef.value?.restoreValidation()
  },
  { immediate: true },
)

const rules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
  username: [
    { required: true, message: 'Username is required', trigger: ['blur', 'input'] },
  ],
  passwordConfirm: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule: FormItemRule, value: string) =>
        value === model.password ? true : new Error('Passwords do not match'),
    },
  ],
}

async function submit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  submitting.value = true
  try {
    const payload: UsrUpdateProfileReq = {
      name: model.name,
      username: model.username,
    }
    // Only send the password when the user actually typed a new one.
    if (model.password) payload.password = model.password
    await authStore.updateProfile(payload)
    model.password = ''
    model.passwordConfirm = ''
    message.success('Profile updated')
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
  <NSpace vertical :size="16" style="max-width: 520px">
    <NCard title="My profile">
      <NForm
        ref="formRef"
        :model="model"
        :rules="rules"
        label-placement="top"
        :disabled="submitting"
      >
        <NFormItem label="Role">
          <NTag :type="profile?.is_admin ? 'success' : 'default'" size="small">
            {{ profile?.is_admin ? 'Administrator' : 'User' }}
          </NTag>
        </NFormItem>
        <NFormItem label="Name" path="name">
          <NInput v-model:value="model.name" placeholder="Full name" clearable />
        </NFormItem>
        <NFormItem label="Username" path="username">
          <NInput
            v-model:value="model.username"
            placeholder="Login username"
            clearable
          />
        </NFormItem>
        <NFormItem label="New password (leave blank to keep)" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            placeholder="Unchanged"
          />
        </NFormItem>
        <NFormItem label="Confirm new password" path="passwordConfirm">
          <NInput
            v-model:value="model.passwordConfirm"
            type="password"
            show-password-on="click"
            placeholder="Repeat new password"
            :disabled="submitting || !model.password"
          />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" :loading="submitting" @click="submit">
            Save changes
          </NButton>
        </NFormItem>
      </NForm>
    </NCard>
  </NSpace>
</template>
