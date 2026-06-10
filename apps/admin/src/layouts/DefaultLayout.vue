<script setup lang="ts">
import { computed, h } from 'vue'
import type { Component } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { NLayout, NLayoutHeader, NLayoutSider, NLayoutContent, NMenu } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const { sidebarCollapsed } = storeToRefs(appStore)

const route = useRoute()
const activeKey = computed(() => (route.name as string | undefined) ?? null)

function renderRouterLink(to: string, label: string): Component {
  return () => h(RouterLink, { to }, { default: () => label })
}

const menuOptions: MenuOption[] = [
  {
    label: renderRouterLink('/', 'Dashboard'),
    key: 'home',
  },
]
</script>

<template>
  <NLayout position="absolute">
    <NLayoutHeader bordered class="app-header">
      <span class="app-header__title">Secret Management System</span>
    </NLayoutHeader>
    <NLayout has-sider position="absolute" style="top: 56px">
      <NLayoutSider
        bordered
        collapse-mode="width"
        :collapsed-width="64"
        :width="240"
        :collapsed="sidebarCollapsed"
        show-trigger
        @update:collapsed="appStore.toggleSidebar"
      >
        <NMenu
          :value="activeKey"
          :collapsed="sidebarCollapsed"
          :collapsed-width="64"
          :options="menuOptions"
        />
      </NLayoutSider>
      <NLayoutContent class="app-content">
        <RouterView />
      </NLayoutContent>
    </NLayout>
  </NLayout>
</template>

<style scoped>
.app-header {
  display: flex;
  align-items: center;
  height: 56px;
  padding: 0 24px;
}

.app-header__title {
  font-size: 18px;
  font-weight: 600;
}

.app-content {
  padding: 24px;
}
</style>
