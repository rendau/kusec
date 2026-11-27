import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    /** Page title shown in `document.title`. */
    title?: string
    /** Accessible without authentication (login page). */
    public?: boolean
    requiresAuth?: boolean
    requiresAdmin?: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { title: 'Sign in', public: true },
  },
  {
    // Consent page of the MCP server's OAuth flow; the MCP client opens it.
    path: '/oauth/authorize',
    name: 'oauth-authorize',
    component: () => import('@/views/OAuthAuthorizeView.vue'),
    meta: { title: 'Authorize access', requiresAuth: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/DefaultLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'home',
        component: () => import('@/views/HomeView.vue'),
        meta: { title: 'Dashboard' },
      },
      {
        path: 'profile',
        name: 'profile',
        component: () => import('@/views/ProfileView.vue'),
        meta: { title: 'My profile' },
      },
      {
        path: 'app',
        name: 'app-home',
        component: () => import('@/views/AppWorkspaceView.vue'),
        meta: { title: 'Applications' },
      },
      {
        path: 'app/:id',
        name: 'app-workspace',
        component: () => import('@/views/AppWorkspaceView.vue'),
        meta: { title: 'Application' },
      },
      {
        path: 'usr',
        name: 'usr-list',
        component: () => import('@/views/UsrListView.vue'),
        meta: { title: 'Users', requiresAdmin: true },
      },
      {
        path: 'api-key',
        name: 'api-key-list',
        component: () => import('@/views/ApiKeyListView.vue'),
        meta: { title: 'API keys' },
      },
      {
        path: 'inactive',
        name: 'inactive',
        component: () => import('@/views/InactiveView.vue'),
        meta: { title: 'Inactive elements', requiresAdmin: true },
      },
      {
        path: 'audit',
        name: 'audit',
        component: () => import('@/views/AuditView.vue'),
        meta: { title: 'Audit' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { title: 'Not Found' },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(_to, _from, savedPosition) {
    return savedPosition ?? { top: 0 }
  },
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  if (!authStore.initialized) {
    // Never rejects and never drops a session that was merely unreachable
    // (server down, no network) — see `initialize`.
    await authStore.initialize()
  }

  // Public pages (login): bounce authenticated users to where they were
  // heading (`redirect`, e.g. after a reload restored the session), else home.
  if (to.meta.public) {
    if (!authStore.isAuthenticated) return true
    const redirect = to.query.redirect
    return typeof redirect === 'string' && redirect.startsWith('/')
      ? redirect
      : { name: 'home' }
  }

  // Protected pages: redirect to login, preserving the target.
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  // Admin-only pages.
  if (to.meta.requiresAdmin && !authStore.isAdmin) {
    return { name: 'home' }
  }

  return true
})

const APP_NAME = 'Secret Management System'

router.afterEach((to) => {
  const title = to.meta.title
  document.title = title ? `${title} · ${APP_NAME}` : APP_NAME
})

export default router
