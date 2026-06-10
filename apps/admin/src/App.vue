<script setup lang="ts">
import { darkTheme, useOsTheme } from 'naive-ui'
import type { GlobalTheme } from 'naive-ui'
import { computed } from 'vue'

import {
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  NNotificationProvider,
  NGlobalStyle,
} from 'naive-ui'

const osTheme = useOsTheme()
const theme = computed<GlobalTheme | null>(() =>
  osTheme.value === 'dark' ? darkTheme : null,
)
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
