<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NInput,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormItemInst, FormItemRule, FormRules } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { ApiError, apiErrorMessage } from '@/api/http'
import type { UsrUpdateProfileReq } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { passwordComplexityError } from '@/utils/password'

import OAuthGrantCard from '@/components/oauth/OAuthGrantCard.vue'

const message = useMessage()
const router = useRouter()
const authStore = useAuthStore()
const { profile } = storeToRefs(authStore)

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

interface FormModel {
  name: string
  username: string
  password: string
  passwordConfirm: string
  currentPassword: string
}

const model = reactive<FormModel>({
  name: '',
  username: '',
  password: '',
  passwordConfirm: '',
  currentPassword: '',
})

const currentPasswordItemRef = ref<FormItemInst | null>(null)
// "Current password is incorrect" comes from the server, not from the form
// rules, so it is shown through the field's controlled status + feedback.
// While there is no such error the props are absent (not `undefined`) and the
// form rules drive the field.
const currentPasswordError = ref('')
const currentPasswordFeedback = computed(() =>
  currentPasswordError.value
    ? { validationStatus: 'error' as const, feedback: currentPasswordError.value }
    : {},
)

// Seed (and re-seed) the form from the loaded profile.
watch(
  profile,
  (value) => {
    model.name = value?.name ?? ''
    model.username = value?.username ?? ''
    model.password = ''
    model.passwordConfirm = ''
    model.currentPassword = ''
    currentPasswordError.value = ''
    formRef.value?.restoreValidation()
  },
  { immediate: true },
)

const usernameChanged = computed(
  () => model.username.trim() !== (profile.value?.username ?? ''),
)

// The backend asks for the current password whenever the password or the
// username changes; a new display name alone does not need it.
const needsCurrentPassword = computed(
  () => Boolean(model.password) || usernameChanged.value,
)

// No longer needed (the change was undone) — drop a leftover error on it.
watch(needsCurrentPassword, (needed) => {
  if (needed) return
  currentPasswordError.value = ''
  currentPasswordItemRef.value?.restoreValidation()
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
  username: [
    { required: true, message: 'Username is required', trigger: ['blur', 'input'] },
  ],
  password: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule: FormItemRule, value: string) => {
        if (!value) return true
        const err = passwordComplexityError(value)
        return err ? new Error(err) : true
      },
    },
  ],
  passwordConfirm: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule: FormItemRule, value: string) =>
        value === model.password ? true : new Error('Passwords do not match'),
    },
  ],
  currentPassword: needsCurrentPassword.value
    ? [
        {
          required: true,
          message: 'Enter your current password to confirm the change',
          trigger: ['blur', 'input'],
        },
      ]
    : [],
}))

async function submit(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  submitting.value = true
  currentPasswordError.value = ''
  try {
    const payload: UsrUpdateProfileReq = { name: model.name }
    // Username and password go out only when they actually change: each of
    // them makes the backend require `current_password`.
    if (usernameChanged.value) payload.username = model.username.trim()
    if (model.password) payload.password = model.password
    if (needsCurrentPassword.value) payload.current_password = model.currentPassword

    const sessionEnded = await authStore.updateProfile(payload)
    if (sessionEnded) {
      // A new password signs the user out everywhere, here included: the
      // store has dropped the session, the login page explains why.
      await router.replace({ name: 'login' })
      return
    }
    model.password = ''
    model.passwordConfirm = ''
    model.currentPassword = ''
    message.success('Profile updated')
    // Возврат туда, откуда пришли (профиль — модальный по смыслу экран).
    router.back()
  } catch (error) {
    if (error instanceof ApiError && error.code === 'current_password_invalid') {
      currentPasswordError.value = 'Current password is incorrect'
    } else {
      message.error(apiErrorMessage(error, 'Unexpected error, please try again'))
    }
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
            :input-props="{ autocomplete: 'username' }"
          />
        </NFormItem>
        <NFormItem label="New password (leave blank to keep)" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            placeholder="Unchanged"
            :input-props="{ autocomplete: 'new-password' }"
          />
        </NFormItem>
        <NFormItem label="Confirm new password" path="passwordConfirm">
          <NInput
            v-model:value="model.passwordConfirm"
            type="password"
            show-password-on="click"
            placeholder="Repeat new password"
            :disabled="submitting || !model.password"
            :input-props="{ autocomplete: 'new-password' }"
          />
        </NFormItem>
        <NFormItem
          ref="currentPasswordItemRef"
          label="Current password"
          path="currentPassword"
          v-bind="currentPasswordFeedback"
        >
          <NSpace vertical :size="4" style="width: 100%">
            <NInput
              v-model:value="model.currentPassword"
              type="password"
              show-password-on="click"
              placeholder="Current password"
              :input-props="{ autocomplete: 'current-password' }"
              @update:value="currentPasswordError = ''"
            />
            <NText depth="3" style="font-size: 12px">
              {{
                model.password
                  ? 'Needed to change the password. You will be signed out on all devices and sign in again with the new one.'
                  : 'Needed only to change your username or password.'
              }}
            </NText>
          </NSpace>
        </NFormItem>
        <NFormItem>
          <NButton type="primary" :loading="submitting" @click="submit">
            Save changes
          </NButton>
        </NFormItem>
      </NForm>
    </NCard>

    <OAuthGrantCard />
  </NSpace>
</template>
