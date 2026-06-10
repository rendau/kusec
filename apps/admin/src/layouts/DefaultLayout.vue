<script setup lang="ts">
import { computed, h } from 'vue'
import type { Component, VNode } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  NDropdown,
  NIcon,
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

// Inline SVG icons (no icon-library dependency). Rendered via NIcon so the
// collapsed sidebar still shows a glyph for every menu item.
function renderIcon(path: string): () => VNode {
  return () =>
    h(NIcon, null, {
      default: () =>
        h(
          'svg',
          { viewBox: '0 0 24 24', width: '1em', height: '1em' },
          [h('path', { d: path, fill: 'currentColor' })],
        ),
    })
}

const icons = {
  dashboard:
    'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z',
  app:
    'M4 8h4V4H4v4zm6 12h4v-4h-4v4zm-6 0h4v-4H4v4zm0-6h4v-4H4v4zm6 0h4v-4h-4v4zm6-10v4h4V4h-4zm-6 4h4V4h-4v4zm6 6h4v-4h-4v4zm0 6h4v-4h-4v4z',
  secret:
    'M12.65 10C11.83 7.67 9.61 6 7 6c-3.31 0-6 2.69-6 6s2.69 6 6 6c2.61 0 4.83-1.67 5.65-4H17v4h4v-4h2v-4H12.65zM7 14c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2z',
  item:
    'M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z',
  user:
    'M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z',
} as const

const menuOptions = computed<MenuOption[]>(() => [
  {
    label: renderRouterLink('/', 'Dashboard'),
    key: 'home',
    icon: renderIcon(icons.dashboard),
  },
  {
    label: renderRouterLink('/app', 'Applications'),
    key: 'app-list',
    icon: renderIcon(icons.app),
  },
  {
    label: renderRouterLink('/secret', 'Secrets'),
    key: 'secret-list',
    icon: renderIcon(icons.secret),
  },
  {
    label: renderRouterLink('/item', 'Items'),
    key: 'item-list',
    icon: renderIcon(icons.item),
  },
  // User management is admin-only.
  ...(authStore.isAdmin
    ? [
        {
          label: renderRouterLink('/usr', 'Users'),
          key: 'usr-list',
          icon: renderIcon(icons.user),
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
