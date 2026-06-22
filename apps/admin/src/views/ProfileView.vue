<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
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
import type { FormInst, FormItemRule, FormRules } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { apiErrorMessage } from '@/api/http'
import { disableTotp } from '@/api/usr'
import type { UsrUpdateProfileReq } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { passwordComplexityError } from '@/utils/password'

import TotpSetupCard from '@/components/usr/TotpSetupCard.vue'

const message = useMessage()
const router = useRouter()
const authStore = useAuthStore()
const { profile } = storeToRefs(authStore)

const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

// ── 2FA (TOTP) ─────────────────────────────────────────────
const enrolling = ref(false)
const disabling = ref(false)
const disableCode = ref('')

async function onTotpEnabled(): Promise<void> {
  enrolling.value = false
  await authStore.refreshProfile()
  message.success('Two-factor authentication enabled')
}

async function submitDisable(): Promise<void> {
  const code = disableCode.value.trim()
  if (!code) {
    message.error('Enter a current code to disable 2FA')
    return
  }
  disabling.value = true
  try {
    await disableTotp(code)
    disableCode.value = ''
    await authStore.refreshProfile()
    message.success('Two-factor authentication disabled')
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to disable 2FA'))
  } finally {
    disabling.value = false
  }
}

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
    // Возврат туда, откуда пришли (профиль — модальный по смыслу экран).
    router.back()
  } catch (error) {
    message.error(apiErrorMessage(error, 'Unexpected error, please try again'))
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

    <NCard title="Two-factor authentication">
      <NSpace align="center" :size="12" style="margin-bottom: 16px">
        <NTag :type="profile?.totp_enabled ? 'success' : 'warning'" size="small">
          {{ profile?.totp_enabled ? 'Enabled' : 'Disabled' }}
        </NTag>
        <NText :depth="3" style="font-size: 13px">
          {{
            profile?.totp_enabled
              ? 'A code from your authenticator app is required at sign-in.'
              : profile?.is_admin
                ? 'Required for administrators — set it up to keep access.'
                : 'Add an extra layer of protection to your account.'
          }}
        </NText>
      </NSpace>

      <!-- Enabled: offer to turn off with a current code. -->
      <template v-if="profile?.totp_enabled">
        <NForm label-placement="top" @submit.prevent="submitDisable">
          <NFormItem label="Current code (to disable)">
            <NInput
              v-model:value="disableCode"
              placeholder="123456"
              maxlength="6"
              :input-props="{ inputmode: 'numeric', autocomplete: 'one-time-code' }"
              style="max-width: 200px"
            />
          </NFormItem>
          <NButton type="error" :loading="disabling" @click="submitDisable">
            Disable 2FA
          </NButton>
        </NForm>
      </template>

      <!-- Disabled: start the enrolment flow. -->
      <template v-else>
        <TotpSetupCard v-if="enrolling" @confirmed="onTotpEnabled" />
        <NButton v-else type="primary" @click="enrolling = true"> Enable 2FA </NButton>
      </template>
    </NCard>
  </NSpace>
</template>
