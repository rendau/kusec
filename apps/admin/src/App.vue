<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { darkTheme, useOsTheme } from 'naive-ui'
import type { GlobalTheme } from 'naive-ui'

import {
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  NNotificationProvider,
  NGlobalStyle,
} from 'naive-ui'

import { useAuthStore } from '@/stores/auth'

const osTheme = useOsTheme()
const theme = computed<GlobalTheme | null>(() =>
  osTheme.value === 'dark' ? darkTheme : null,
)

const router = useRouter()
const authStore = useAuthStore()

// Dispatched by the API layer when a request fails auth and silent
// renewal is impossible — drop the session and return to login.
function onAuthRequired(): void {
  authStore.logout()
  void router.push({ name: 'login' })
}

onMounted(() => {
  window.addEventListener('auth:required', onAuthRequired)
})

onUnmounted(() => {
  window.removeEventListener('auth:required', onAuthRequired)
})
</script>

<template>
  <NConfigProvider :theme="theme">
    <NLoadingBarProvider>
      <NMessageProvider>
        <NNotificationProvider>
          <NDialogProvider>
            <NGlobalStyle />
            <RouterView />
          </NDialogProvider>
        </NNotificationProvider>
      </NMessageProvider>
    </NLoadingBarProvider>
  </NConfigProvider>
</template>
