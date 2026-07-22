<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import type { Component as VueComponent } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NDropdown,
  NIcon,
  NLayout,
  NLayoutHeader,
  NLayoutSider,
  NLayoutContent,
  NMenu,
  NTag,
  NTooltip,
  useDialog,
  useThemeVars,
} from 'naive-ui'
import type { DropdownOption, MenuOption } from 'naive-ui'
import { CloudDownload, CloudUpload, Menu2 } from '@vicons/tabler'
import { storeToRefs } from 'pinia'

import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useKubeSync } from '@/composables/useKubeSync'
import { useBreakpoint } from '@/composables/useBreakpoint'

import AppNavSidebar from '@/components/app/AppNavSidebar.vue'
import KubeImportModal from '@/components/kube/KubeImportModal.vue'
import KubeSyncReportModal from '@/components/kube/KubeSyncReportModal.vue'
import { useAppsStore } from '@/stores/apps'

const appStore = useAppStore()
const { sidebarCollapsed, sidebarWidth } = storeToRefs(appStore)
const themeVars = useThemeVars()

// On small screens the sidebar becomes an overlay drawer toggled from the
// header, instead of an always-on resizable column.
const { isMobile } = useBreakpoint()
const mobileNavOpen = ref(false)

// Desktop honours the stored collapse preference; mobile uses the drawer state.
const siderCollapsed = computed(() =>
  isMobile.value ? !mobileNavOpen.value : sidebarCollapsed.value,
)

function onSiderCollapsed(collapsed: boolean): void {
  if (isMobile.value) {
    mobileNavOpen.value = !collapsed
  } else {
    appStore.toggleSidebar()
  }
}

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

// Global "sync all accessible apps" action — lives in the header so it stays
// reachable from any page (per-app sync lives inside each app's workspace).
// The report state is shared across call sites, so this single modal also
// shows results of per-app syncs triggered from a workspace.
const { syncing, confirmSync, report, reportVisible } = useKubeSync()

// Importing cluster secrets creates applications, so it is admin-only.
const appsStore = useAppsStore()
const showImport = ref(false)

async function onImported(): Promise<void> {
  await appsStore.refresh()
}

const route = useRoute()

// Auto-close the mobile drawer after navigating to another page/app.
watch(
  () => route.fullPath,
  () => {
    mobileNavOpen.value = false
  },
)

// Primary nav lives in the header now; the sidebar is dedicated to applications.
const navActiveKey = computed(() => {
  const name = route.name as string | undefined
  if (name === 'usr-list') return 'users'
  if (name === 'api-key-list') return 'api-keys'
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
  { label: renderRouterLink('/api-key', 'API keys'), key: 'api-keys' },
])

const profileName = computed(
  () => authStore.profile?.name || authStore.profile?.username || 'User',
)
const profileRole = computed(() => (authStore.isAdmin ? 'admin' : 'user'))

const userMenuOptions = computed<DropdownOption[]>(() => [
  // On mobile the header nav is hidden, so surface those links here.
  ...(isMobile.value
    ? [
        { label: 'Dashboard', key: 'home' },
        ...(authStore.isAdmin ? [{ label: 'Users', key: 'users' }] : []),
        { label: 'API keys', key: 'api-keys' },
        { type: 'divider', key: 'd0' } as DropdownOption,
      ]
    : []),
  { label: 'My profile', key: 'profile' },
  { type: 'divider', key: 'd1' },
  { label: 'Log out', key: 'logout' },
])

function onUserMenuSelect(key: string | number): void {
  if (key === 'home') {
    void router.push({ name: 'home' })
  } else if (key === 'users') {
    void router.push({ name: 'usr-list' })
  } else if (key === 'api-keys') {
    void router.push({ name: 'api-key-list' })
  } else if (key === 'profile') {
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
        <NButton
          quaternary
          circle
          class="app-header__burger"
          aria-label="Toggle navigation"
          @click="mobileNavOpen = !mobileNavOpen"
        >
          <template #icon>
            <NIcon :component="Menu2" />
          </template>
        </NButton>
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
      <div class="app-header__right">
        <NTooltip v-if="authStore.isAdmin">
          <template #trigger>
            <NButton
              quaternary
              circle
              aria-label="Import secrets from cluster"
              @click="showImport = true"
            >
              <template #icon>
                <NIcon :component="CloudDownload" />
              </template>
            </NButton>
          </template>
          Import secrets from cluster
        </NTooltip>
        <NTooltip>
          <template #trigger>
            <NButton
              quaternary
              circle
              :loading="syncing"
              aria-label="Sync all to Kubernetes"
              @click="confirmSync()"
            >
              <template #icon>
                <NIcon :component="CloudUpload" />
              </template>
            </NButton>
          </template>
          Sync all to Kubernetes
        </NTooltip>
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
      </div>
    </NLayoutHeader>
    <NLayout has-sider position="absolute" style="top: 56px">
      <NLayoutSider
        bordered
        collapse-mode="width"
        :collapsed-width="0"
        :width="isMobile ? 280 : sidebarWidth"
        :collapsed="siderCollapsed"
        :position="isMobile ? 'absolute' : 'static'"
        :show-trigger="isMobile ? false : 'arrow-circle'"
        :class="{ 'sider--overlay': isMobile }"
        @update:collapsed="onSiderCollapsed"
      >
        <div class="sider-wrap">
          <AppNavSidebar />
          <div
            v-if="!isMobile && !sidebarCollapsed"
            class="sider-resizer"
            @mousedown="startResize"
          />
        </div>
      </NLayoutSider>
      <div
        v-if="isMobile && mobileNavOpen"
        class="sider-backdrop"
        @click="mobileNavOpen = false"
      />
      <NLayoutContent class="app-content">
        <RouterView v-slot="{ Component, route: current }">
          <Transition name="page" mode="out-in">
            <component :is="Component" :key="current.path" />
          </Transition>
        </RouterView>
      </NLayoutContent>
    </NLayout>

    <KubeImportModal v-model:show="showImport" @imported="onImported" />
    <KubeSyncReportModal v-model:show="reportVisible" :report="report" />
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

/* Hamburger toggles the overlay drawer; shown only on small screens. */
.app-header__burger {
  display: none;
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

.app-header__right {
  display: flex;
  align-items: center;
  gap: 8px;
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

/* The mobile drawer floats above the content; the backdrop dims the rest. */
.sider--overlay {
  z-index: 20;
}

.sider-backdrop {
  position: absolute;
  inset: 0;
  z-index: 15;
  background: rgba(0, 0, 0, 0.45);
}

@media (max-width: 768px) {
  .app-header {
    padding: 0 12px;
    gap: 8px;
  }

  .app-header__left {
    gap: 8px;
  }

  .app-header__burger {
    display: inline-flex;
  }

  /* The horizontal page nav moves into the sidebar drawer on mobile. */
  .app-header__nav {
    display: none;
  }

  .app-header__user-name {
    display: none;
  }

  .app-content {
    padding: 16px 12px;
  }
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
