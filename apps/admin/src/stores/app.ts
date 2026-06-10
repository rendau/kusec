import { ref } from 'vue'
import { defineStore } from 'pinia'

/**
 * Global application UI state (sidebar, layout preferences).
 * Business-domain state lives in feature-specific stores.
 */
export const useAppStore = defineStore('app', () => {
  const sidebarCollapsed = ref(false)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return {
    sidebarCollapsed,
    toggleSidebar,
  }
})
