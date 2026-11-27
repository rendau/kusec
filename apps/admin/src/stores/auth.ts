import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import type { UsrMain, UsrUpdateProfileReq } from '@/api/types'
import { clearSession, getToken } from '@/api/auth-session'
import { useAppsStore } from '@/stores/apps'
import {
  getProfile,
  login as apiLogin,
  logout as apiLogout,
  updateProfile as apiUpdateProfile,
} from '@/api/usr'
import { usrAccessKind } from '@/utils/usrAccess'
import type { UsrAccessKind } from '@/utils/usrAccess'

interface LoginPayload {
  username: string
  password: string
}

/**
 * Why the user was sent to the login page, shown there once:
 *   - `password_changed`    — the password was changed, sign in with the new one;
 *   - `session_unavailable` — the stored session could not be restored because
 *     the server did not answer properly; the session itself is kept.
 */
export type LoginNotice = 'password_changed' | 'session_unavailable'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(getToken())
  const profile = ref<UsrMain | null>(null)
  const initialized = ref(false)
  const loading = ref(false)
  const loginNotice = ref<LoginNotice | null>(null)

  const isAuthenticated = computed(() => Boolean(token.value) && Boolean(profile.value))
  const isAdmin = computed(() => Boolean(profile.value?.is_admin))

  /**
   * Which applications the current user can open (`null` before the profile
   * is loaded). `none` is a real state: a non-admin without `all_apps` and
   * with an empty `app_ids` sees no application at all.
   */
  const appAccess = computed<UsrAccessKind | null>(() =>
    profile.value ? usrAccessKind(profile.value) : null,
  )

  function syncToken(): void {
    token.value = getToken()
  }

  /**
   * Restore the session on app start: load the profile if a token exists.
   * Never rejects. When the profile cannot be loaded there are two cases:
   *   - the backend refused the session — the API layer has already cleared
   *     the tokens, the rest is dropped here;
   *   - it simply could not be asked (server or its database down, no
   *     network, timeout) — the tokens are **kept**: the user lands on the
   *     login page with a notice, and a reload restores the session.
   */
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
    } catch {
      if (getToken()) {
        loginNotice.value = 'session_unavailable'
      } else {
        dropSession()
      }
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

  async function login(payload: LoginPayload): Promise<void> {
    loading.value = true
    try {
      await apiLogin(payload.username, payload.password)
      await completeSession()
    } finally {
      loading.value = false
    }
  }

  async function refreshProfile(): Promise<void> {
    try {
      profile.value = await getProfile()
    } finally {
      // The token pair may have been replaced outside the store (silent
      // renewal) — pick it up even when the profile call fails.
      syncToken()
    }
  }

  /**
   * Forget the session in this browser only — no request to the server. For
   * a session that is already dead there (`auth:required`, a password
   * change). A sign-out the user asked for goes through `logout`. Never call
   * it for a request that merely failed: only a refused refresh ends a session.
   */
  function dropSession(): void {
    clearSession()
    token.value = ''
    profile.value = null
    initialized.value = true
    // Drop the previous session's apps so the next login reloads them.
    useAppsStore().reset()
  }

  /**
   * Sign out on the user's request: revoke the tokens on the server (which
   * ends the session on every device), then drop the local session. The
   * local part happens no matter how the request ends — offline, timeout or
   * an already dead token must not keep the user signed in here.
   */
  async function logout(): Promise<void> {
    try {
      if (getToken()) await apiLogout()
    } catch {
      // Ignored on purpose: the local sign-out below is what the user asked for.
    } finally {
      dropSession()
    }
  }

  /**
   * Save the own profile. Resolves to `true` when the change ended the
   * session: the backend revokes every token of the user on a password
   * change, so the local session is dropped here (re-reading the profile
   * with the dead token would only raise `auth:required`) and the caller
   * just has to open the login page.
   */
  async function updateProfile(payload: UsrUpdateProfileReq): Promise<boolean> {
    await apiUpdateProfile(payload)
    if (payload.password) {
      dropSession()
      loginNotice.value = 'password_changed'
      return true
    }
    await refreshProfile()
    return false
  }

  /** Read the pending login-page notice and clear it (shown once). */
  function takeLoginNotice(): LoginNotice | null {
    const notice = loginNotice.value
    loginNotice.value = null
    return notice
  }

  return {
    token,
    profile,
    initialized,
    loading,
    isAuthenticated,
    isAdmin,
    appAccess,
    initialize,
    login,
    refreshProfile,
    updateProfile,
    dropSession,
    logout,
    takeLoginNotice,
  }
})
