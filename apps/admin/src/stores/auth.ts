import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import type { UsrMain, UsrUpdateProfileReq } from '@/api/types'
import { getToken } from '@/api/auth-session'
import { useAppsStore } from '@/stores/apps'
import {
  getProfile,
  login as apiLogin,
  logout as apiLogout,
  updateProfile as apiUpdateProfile,
} from '@/api/usr'

interface LoginPayload {
  username: string
  password: string
  totpCode?: string
}

/**
 * Outcome of a login attempt:
 *   - `ok`: a session was established;
 *   - `totp_required`: password accepted, re-submit with a TOTP code;
 *   - `totp_setup_required`: (admin) must enable 2FA first; `setupToken`
 *     authorises the enroll/confirm endpoints.
 */
export type LoginOutcome =
  | { status: 'ok' }
  | { status: 'totp_required' }
  | { status: 'totp_setup_required'; setupToken: string }

export const useAuthStore = defineStore('auth', () => {
  const token = ref(getToken())
  const profile = ref<UsrMain | null>(null)
  const initialized = ref(false)
  const loading = ref(false)

  const isAuthenticated = computed(() => Boolean(token.value) && Boolean(profile.value))
  const isAdmin = computed(() => Boolean(profile.value?.is_admin))

  function syncToken(): void {
    token.value = getToken()
  }

  /** Restore the session on app start: load the profile if a token exists. */
  async function initialize(): Promise<void> {
    if (initialized.value) {
      return
    }
    syncToken()
    if (!token.value) {
      initialized.value = true
      return
    }
    try {
      profile.value = await getProfile()
    } finally {
      syncToken()
      initialized.value = true
    }
  }

  /** Load the profile from a freshly persisted token pair. */
  async function completeSession(): Promise<void> {
    syncToken()
    profile.value = await getProfile()
    initialized.value = true
  }

  async function login(payload: LoginPayload): Promise<LoginOutcome> {
    loading.value = true
    try {
      const rep = await apiLogin(payload.username, payload.password, payload.totpCode)
      if (rep.totp_setup_required) {
        return { status: 'totp_setup_required', setupToken: rep.setup_token ?? '' }
      }
      if (rep.totp_required) {
        return { status: 'totp_required' }
      }
      await completeSession()
      return { status: 'ok' }
    } finally {
      loading.value = false
    }
  }

  // Finish a 2FA enrolment confirmed elsewhere (TotpSetupCard already persisted
  // a fresh token pair) — just load the profile to open the session.
  async function completeTotpSetup(): Promise<void> {
    await completeSession()
  }

  async function refreshProfile(): Promise<void> {
    profile.value = await getProfile()
    syncToken()
  }

  async function updateProfile(payload: UsrUpdateProfileReq): Promise<void> {
    await apiUpdateProfile(payload)
    await refreshProfile()
  }

  function logout(): void {
    apiLogout()
    token.value = ''
    profile.value = null
    initialized.value = true
    // Drop the previous session's apps so the next login reloads them.
    useAppsStore().reset()
  }

  return {
    token,
    profile,
    initialized,
    loading,
    isAuthenticated,
    isAdmin,
    initialize,
    login,
    completeTotpSetup,
    refreshProfile,
    updateProfile,
    logout,
  }
})
