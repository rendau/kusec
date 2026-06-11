<script setup lang="ts">
import { computed, h } from 'vue'
import type { Component as VueComponent } from 'vue'
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
  useThemeVars,
} from 'naive-ui'
import type { DropdownOption, MenuOption } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

import AppNavSidebar from '@/components/app/AppNavSidebar.vue'

const appStore = useAppStore()
const { sidebarCollapsed, sidebarWidth } = storeToRefs(appStore)
const themeVars = useThemeVars()

// Drag the right edge of the sidebar to resize it.
function startResize(event: MouseEvent): void {
  event.preventDefault()
  const startX = event.clientX
  const startWidth = sidebarWidth.value

  function onMove(e: MouseEvent): void {
    appStore.setSidebarWidth(startWidth + (e.clientX - startX))
  }

  function onUp(): void {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    document.body.style.userSelect = ''
    document.body.style.cursor = ''
  }

  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'col-resize'
}

const authStore = useAuthStore()
const router = useRouter()
const dialog = useDialog()

const route = useRoute()

// Primary nav lives in the header now; the sidebar is dedicated to applications.
const navActiveKey = computed(() => {
  const name = route.name as string | undefined
  if (name === 'usr-list') return 'users'
  if (name === 'home') return 'home'
  return null
})

function renderRouterLink(to: string, label: string): VueComponent {
  return () => h(RouterLink, { to }, { default: () => label })
}

const navOptions = computed<MenuOption[]>(() => [
  { label: renderRouterLink('/', 'Dashboard'), key: 'home' },
  ...(authStore.isAdmin
    ? [{ label: renderRouterLink('/usr', 'Users'), key: 'users' }]
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
      <div class="app-header__left">
        <RouterLink to="/" class="app-header__brand">
          <img src="/favicon.svg" alt="" class="app-header__logo" />
          <span class="app-header__title">Kusec</span>
        </RouterLink>
        <NMenu
          mode="horizontal"
          :value="navActiveKey"
          :options="navOptions"
          class="app-header__nav"
        />
      </div>
      <NDropdown trigger="click" :options="userMenuOptions" @select="onUserMenuSelect">
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
        :collapsed-width="0"
        :width="sidebarWidth"
        :collapsed="sidebarCollapsed"
        show-trigger="arrow-circle"
        @update:collapsed="appStore.toggleSidebar"
      >
        <div class="sider-wrap">
          <AppNavSidebar />
          <div
            v-if="!sidebarCollapsed"
            class="sider-resizer"
            @mousedown="startResize"
          />
        </div>
      </NLayoutSider>
      <NLayoutContent class="app-content">
        <RouterView v-slot="{ Component, route: current }">
          <Transition name="page" mode="out-in">
            <component :is="Component" :key="current.path" />
          </Transition>
        </RouterView>
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

.app-header__left {
  display: flex;
  align-items: center;
  gap: 24px;
  min-width: 0;
}

.app-header__brand {
  display: flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  color: inherit;
}

.app-header__logo {
  width: 24px;
  height: 24px;
  display: block;
}

.app-header__title {
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
}

.app-header__nav {
  background: transparent;
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

.sider-wrap {
  position: relative;
  height: 100%;
}

/* Drag handle on the sidebar's right edge. */
.sider-resizer {
  position: absolute;
  top: 0;
  right: 0;
  width: 6px;
  height: 100%;
  cursor: col-resize;
  z-index: 1;
}

.sider-resizer:hover,
.sider-resizer:active {
  background: v-bind('`${themeVars.primaryColor}4d`');
}
</style>
