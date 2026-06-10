<script setup lang="ts">
import { useRouter } from 'vue-router'
import { NButton, NResult } from 'naive-ui'

const router = useRouter()
</script>

<template>
  <NResult status="404" title="404" description="Page not found">
    <template #footer>
      <NButton @click="router.push('/')">Back to dashboard</NButton>
    </template>
  </NResult>
</template>
