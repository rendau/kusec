<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert, NButton, NCard, NIcon, NSpin } from 'naive-ui'
import { EyeOff, Pencil } from '@vicons/tabler'
import { storeToRefs } from 'pinia'

import { apiErrorMessage } from '@/api/http'
import { authorize, describeAuthorization } from '@/api/oauth'
import type { OAuthAuthorizeInfoRep, OAuthAuthorizeReq } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const { profile } = storeToRefs(useAuthStore())

function queryParam(name: string): string {
  const value = route.query[name]
  return typeof value === 'string' ? value : ''
}

// The authorization request goes to the backend as is: it checks the client
// and the redirect before anything is shown or redirected.
const request = computed<OAuthAuthorizeReq>(() => ({
  response_type: queryParam('response_type'),
  client_id: queryParam('client_id'),
  redirect_uri: queryParam('redirect_uri'),
  state: queryParam('state'),
  code_challenge: queryParam('code_challenge'),
  code_challenge_method: queryParam('code_challenge_method'),
  scope: queryParam('scope'),
  resource: queryParam('resource'),
}))

const info = ref<OAuthAuthorizeInfoRep | null>(null)
const loading = ref(true)
const errorMessage = ref('')
const deciding = ref<'' | 'allow' | 'deny'>('')
const redirecting = ref(false)

async function decide(approve: boolean): Promise<void> {
  deciding.value = approve ? 'allow' : 'deny'
  errorMessage.value = ''
  try {
    const rep = await authorize(request.value, approve)
    redirecting.value = true
    window.location.assign(rep.redirect_url)
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, 'Unable to complete the authorization')
    deciding.value = ''
  }
}

onMounted(async () => {
  try {
    info.value = await describeAuthorization(request.value)
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, 'Invalid authorization request')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <main class="consent-page">
    <NCard class="consent-card">
      <div class="consent-brand">
        <img src="/mark.svg" alt="" class="consent-brand__logo" />
        <div>
          <div class="consent-brand__name">Kusec</div>
          <div class="consent-brand__sub">Authorize access to the MCP server</div>
        </div>
      </div>

      <div v-if="loading || redirecting" class="consent-loading">
        <NSpin size="medium" />
        <span v-if="redirecting" class="consent-muted">
          Returning to {{ info?.redirect_host }}…
        </span>
      </div>

      <template v-else-if="info">
        <p class="consent-lead">
          <strong>{{ info.client_name }}</strong> wants to connect to Kusec as
          <strong>{{ profile?.username }}</strong
          >.
        </p>

        <ul class="consent-scopes">
          <li>
            <NIcon :component="EyeOff" class="consent-scopes__icon" />
            <span>Read apps, secrets and configmaps. Secret values stay hidden.</span>
          </li>
          <li>
            <NIcon :component="Pencil" class="consent-scopes__icon" />
            <span>
              Create and update secrets and configmaps in your apps and sync them to the
              cluster, with your permissions.
            </span>
          </li>
        </ul>

        <p class="consent-muted consent-note">
          After your decision you return to <code>{{ info.redirect_host }}</code
          >. Allow only if you started connecting this app yourself. You can disconnect
          it any time in My profile.
        </p>

        <div class="consent-actions">
          <NButton
            :disabled="Boolean(deciding)"
            :loading="deciding === 'deny'"
            @click="decide(false)"
          >
            Deny
          </NButton>
          <NButton
            type="primary"
            :disabled="Boolean(deciding)"
            :loading="deciding === 'allow'"
            @click="decide(true)"
          >
            Allow
          </NButton>
        </div>
      </template>

      <NAlert v-if="errorMessage" class="consent-alert" type="error" :show-icon="false">
        {{ errorMessage }}
      </NAlert>

      <RouterLink v-if="!loading && !info" :to="{ name: 'home' }" class="consent-home">
        Go to the dashboard
      </RouterLink>
    </NCard>
  </main>
</template>

<style scoped>
.consent-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  min-height: 100dvh;
  padding: 24px;
}

.consent-card {
  width: 100%;
  max-width: 440px;
}

.consent-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.consent-brand__logo {
  width: 36px;
  height: 36px;
}

.consent-brand__name {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.2;
}

.consent-brand__sub {
  font-size: 12px;
  opacity: 0.65;
}

.consent-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 32px 0;
  font-size: 13px;
}

.consent-muted {
  opacity: 0.7;
}

.consent-lead {
  margin: 0 0 16px;
  font-size: 15px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.consent-scopes {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0 0 16px;
  padding: 12px 14px;
  list-style: none;
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.25));
  border-radius: 8px;
  font-size: 13.5px;
}

.consent-scopes li {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.consent-scopes__icon {
  flex-shrink: 0;
  margin-top: 2px;
  opacity: 0.6;
}

.consent-note {
  margin: 0 0 20px;
  font-size: 12.5px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.consent-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.consent-alert {
  margin-top: 16px;
}

.consent-home {
  display: inline-block;
  margin-top: 14px;
  font-size: 13px;
}
</style>
