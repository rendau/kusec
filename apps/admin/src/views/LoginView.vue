<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NAlert, NButton, NCard, NForm, NFormItem, NInput, NSpin } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { apiErrorMessage } from '@/api/http'
import { createUser, getBootstrapStatus } from '@/api/usr'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()

const formRef = ref<FormInst | null>(null)

// Единая reactive-модель формы: NForm валидирует model[path], поэтому
// значения должны лежать в модели строками, а не вложенными ref'ами.
const model = reactive({
  name: '',
  username: '',
  password: '',
})

const errorMessage = ref('')
const successMessage = ref('')

const statusLoading = ref(true)
const bootstrapAvailable = ref(false)
const bootstrapLoading = ref(false)

const loginRules: FormRules = {
  username: [
    { required: true, message: 'Username is required', trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: 'Password is required', trigger: ['blur', 'input'] },
  ],
}

const bootstrapRules: FormRules = {
  ...loginRules,
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
}

async function submitLogin(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  errorMessage.value = ''
  successMessage.value = ''
  try {
    await authStore.login({
      username: model.username,
      password: model.password,
    })
    const redirect =
      typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(redirect)
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, 'Unable to sign in')
  }
}

async function submitBootstrap(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  errorMessage.value = ''
  successMessage.value = ''
  bootstrapLoading.value = true
  try {
    await createUser({
      active: true,
      is_admin: true,
      name: model.name,
      username: model.username,
      password: model.password,
    })
    successMessage.value = 'First administrator created. You can sign in now.'
    bootstrapAvailable.value = false
    model.password = ''
    formRef.value?.restoreValidation()
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, 'Unable to create first admin')
  } finally {
    bootstrapLoading.value = false
  }
}

onMounted(async () => {
  try {
    const status = await getBootstrapStatus()
    bootstrapAvailable.value = status.can_create_first_admin
  } catch {
    bootstrapAvailable.value = false
  } finally {
    statusLoading.value = false
  }
})
</script>

<template>
  <main class="login-page">
    <NCard class="login-card">
      <div class="login-brand">
        <img src="/favicon.svg" alt="" class="login-brand__logo" />
        <div>
          <div class="login-brand__name">Kusec</div>
          <div class="login-brand__sub">
            {{ bootstrapAvailable ? 'First admin setup' : 'Secret Management System' }}
          </div>
        </div>
      </div>

      <NSpin v-if="statusLoading" />

      <NForm
        v-else-if="bootstrapAvailable"
        ref="formRef"
        :model="model"
        :rules="bootstrapRules"
        @submit.prevent="submitBootstrap"
      >
        <NFormItem label="Name" path="name">
          <NInput
            v-model:value="model.name"
            autocomplete="name"
            placeholder="Full name"
          />
        </NFormItem>
        <NFormItem label="Username" path="username">
          <NInput
            v-model:value="model.username"
            autocomplete="username"
            placeholder="Username"
          />
        </NFormItem>
        <NFormItem label="Password" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            autocomplete="new-password"
            placeholder="Password"
          />
        </NFormItem>
        <NButton :loading="bootstrapLoading" type="primary" attr-type="submit" block>
          Create admin
        </NButton>
        <p class="login-hint">Available only on first setup, while no users exist.</p>
      </NForm>

      <NForm
        v-else
        ref="formRef"
        :model="model"
        :rules="loginRules"
        @submit.prevent="submitLogin"
      >
        <NFormItem label="Username" path="username">
          <NInput
            v-model:value="model.username"
            autocomplete="username"
            placeholder="Username"
          />
        </NFormItem>
        <NFormItem label="Password" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            autocomplete="current-password"
            placeholder="Password"
          />
        </NFormItem>
        <NButton :loading="authStore.loading" type="primary" attr-type="submit" block>
          Sign in
        </NButton>
      </NForm>

      <NAlert v-if="errorMessage" class="login-alert" type="error" :show-icon="false">
        {{ errorMessage }}
      </NAlert>
      <NAlert
        v-if="successMessage"
        class="login-alert"
        type="success"
        :show-icon="false"
      >
        {{ successMessage }}
      </NAlert>
    </NCard>
  </main>
</template>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
}

.login-card {
  width: 100%;
  max-width: 380px;
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.login-brand__logo {
  width: 36px;
  height: 36px;
}

.login-brand__name {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.2;
}

.login-brand__sub {
  font-size: 12px;
  opacity: 0.65;
}

.login-alert {
  margin-top: 16px;
}

.login-hint {
  margin: 12px 0 0;
  font-size: 12px;
  opacity: 0.7;
}
</style>
