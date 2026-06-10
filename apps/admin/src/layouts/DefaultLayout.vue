<script setup lang="ts">
import { computed, h } from 'vue'
import type { Component } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  NDropdown,
  NLayout,
  NLayoutHeader,
  NLayoutSider,
  NLayoutContent,
  NMenu,
  NTag,
  useDialog,
} from 'naive-ui'
import type { DropdownOption, MenuOption } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const appStore = useAppStore()
const { sidebarCollapsed } = storeToRefs(appStore)

const authStore = useAuthStore()
const router = useRouter()
const dialog = useDialog()

const route = useRoute()
const activeKey = computed(() => (route.name as string | undefined) ?? null)

function renderRouterLink(to: string, label: string): Component {
  return () => h(RouterLink, { to }, { default: () => label })
}

const menuOptions = computed<MenuOption[]>(() => [
  {
    label: renderRouterLink('/', 'Dashboard'),
    key: 'home',
  },
  {
    label: renderRouterLink('/app', 'Applications'),
    key: 'app-list',
  },
  {
    label: renderRouterLink('/secret', 'Secrets'),
    key: 'secret-list',
  },
  {
    label: renderRouterLink('/item', 'Items'),
    key: 'item-list',
  },
  // User management is admin-only.
  ...(authStore.isAdmin
    ? [
        {
          label: renderRouterLink('/usr', 'Users'),
          key: 'usr-list',
        },
      ]
    : []),
])

const profileName = computed(
  () => authStore.profile?.name || authStore.profile?.username || 'User',
)
const profileRole = computed(() => (authStore.isAdmin ? 'admin' : 'user'))

const userMenuOptions: DropdownOption[] = [
  { label: 'My profile', key: 'profile' },
  { type: 'divider', key: 'd1' },
  { label: 'Log out', key: 'logout' },
]

function onUserMenuSelect(key: string | number): void {
  if (key === 'profile') {
    void router.push({ name: 'profile' })
  } else if (key === 'logout') {
    confirmLogout()
  }
}

function confirmLogout(): void {
  dialog.warning({
    title: 'Log out',
    content: 'Log out from the current session?',
    positiveText: 'Log out',
    negativeText: 'Cancel',
    onPositiveClick: () => {
      authStore.logout()
      void router.push({ name: 'login' })
    },
  })
}
</script>

<template>
  <NLayout position="absolute">
    <NLayoutHeader bordered class="app-header">
      <span class="app-header__title">Secret Management System</span>
      <NDropdown
        trigger="click"
        :options="userMenuOptions"
        @select="onUserMenuSelect"
      >
        <button class="app-header__user" type="button">
          <span class="app-header__user-name">{{ profileName }}</span>
          <NTag size="small" :type="authStore.isAdmin ? 'success' : 'default'">
            {{ profileRole }}
          </NTag>
        </button>
      </NDropdown>
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
  justify-content: space-between;
  height: 56px;
  padding: 0 24px;
}

.app-header__title {
  font-size: 18px;
  font-weight: 600;
}

.app-header__user {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  background: transparent;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  color: inherit;
  font: inherit;
}

.app-header__user:hover {
  background: rgba(128, 128, 128, 0.12);
}

.app-content {
  padding: 24px;
}
</style>
