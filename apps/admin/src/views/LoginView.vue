<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NCard,
  NForm,
  NFormItem,
  NInput,
  NSpin,
} from 'naive-ui'

import { ApiError } from '@/api/http'
import { createUser, getBootstrapStatus } from '@/api/usr'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()

const username = ref('')
const password = ref('')
const name = ref('')
const errorMessage = ref('')
const successMessage = ref('')

const statusLoading = ref(true)
const bootstrapAvailable = ref(false)
const bootstrapLoading = ref(false)

async function submitLogin(): Promise<void> {
  errorMessage.value = ''
  successMessage.value = ''
  try {
    await authStore.login({
      username: username.value,
      password: password.value,
    })
    const redirect =
      typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(redirect)
  } catch (error) {
    errorMessage.value =
      error instanceof ApiError ? error.message : 'Unable to sign in'
  }
}

async function submitBootstrap(): Promise<void> {
  errorMessage.value = ''
  successMessage.value = ''
  bootstrapLoading.value = true
  try {
    await createUser({
      active: true,
      is_admin: true,
      name: name.value,
      username: username.value,
      password: password.value,
    })
    successMessage.value =
      'First administrator created. You can sign in now.'
    bootstrapAvailable.value = false
    password.value = ''
  } catch (error) {
    errorMessage.value =
      error instanceof ApiError ? error.message : 'Unable to create first admin'
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
    <NCard class="login-card" :title="bootstrapAvailable ? 'First admin setup' : 'Secret Management System'">
      <NSpin v-if="statusLoading" />

      <NForm v-else-if="bootstrapAvailable" @submit.prevent="submitBootstrap">
        <NFormItem label="Name" path="name">
          <NInput v-model:value="name" autocomplete="name" placeholder="Full name" />
        </NFormItem>
        <NFormItem label="Username" path="username">
          <NInput v-model:value="username" autocomplete="username" placeholder="Username" />
        </NFormItem>
        <NFormItem label="Password" path="password">
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            autocomplete="new-password"
            placeholder="Password"
          />
        </NFormItem>
        <NButton
          :loading="bootstrapLoading"
          type="primary"
          attr-type="submit"
          block
        >
          Create admin
        </NButton>
        <p class="login-hint">
          Available only on first setup, while no users exist.
        </p>
      </NForm>

      <NForm v-else @submit.prevent="submitLogin">
        <NFormItem label="Username" path="username">
          <NInput v-model:value="username" autocomplete="username" placeholder="Username" />
        </NFormItem>
        <NFormItem label="Password" path="password">
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            autocomplete="current-password"
            placeholder="Password"
          />
        </NFormItem>
        <NButton
          :loading="authStore.loading"
          type="primary"
          attr-type="submit"
          block
        >
          Sign in
        </NButton>
      </NForm>

      <NAlert
        v-if="errorMessage"
        class="login-alert"
        type="error"
        :show-icon="false"
      >
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

.login-alert {
  margin-top: 16px;
}

.login-hint {
  margin: 12px 0 0;
  font-size: 12px;
  opacity: 0.7;
}
</style>
