import { ref } from 'vue'
import { defineStore } from 'pinia'

const WIDTH_STORAGE_KEY = 'kusec_admin_sidebar_width'
export const SIDEBAR_MIN_WIDTH = 200
export const SIDEBAR_MAX_WIDTH = 520
const SIDEBAR_DEFAULT_WIDTH = 260

function readStoredWidth(): number {
  const raw = Number(localStorage.getItem(WIDTH_STORAGE_KEY))
  if (Number.isFinite(raw) && raw >= SIDEBAR_MIN_WIDTH && raw <= SIDEBAR_MAX_WIDTH) {
    return raw
  }
  return SIDEBAR_DEFAULT_WIDTH
}

/**
 * Global application UI state (sidebar, layout preferences).
 * Business-domain state lives in feature-specific stores.
 */
export const useAppStore = defineStore('app', () => {
  const sidebarCollapsed = ref(false)
  const sidebarWidth = ref(readStoredWidth())

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setSidebarWidth(width: number) {
    const clamped = Math.min(
      SIDEBAR_MAX_WIDTH,
      Math.max(SIDEBAR_MIN_WIDTH, Math.round(width)),
    )
    sidebarWidth.value = clamped
    localStorage.setItem(WIDTH_STORAGE_KEY, String(clamped))
  }

  return {
    sidebarCollapsed,
    sidebarWidth,
    toggleSidebar,
    setSidebarWidth,
  }
})
