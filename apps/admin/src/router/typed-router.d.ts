import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /** Document title segment shown in the browser tab. */
    title?: string
    /** Accessible without authentication (e.g. the login page). */
    public?: boolean
    /** Requires an authenticated session. */
    requiresAuth?: boolean
    /** Requires the authenticated user to be an admin. */
    requiresAdmin?: boolean
  }
}
