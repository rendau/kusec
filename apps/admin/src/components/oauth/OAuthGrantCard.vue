<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NEmpty, NPopconfirm, NSpin, NText, useMessage } from 'naive-ui'

import { apiErrorMessage } from '@/api/http'
import { deleteOAuthGrant, listOAuthGrants } from '@/api/oauth'
import type { OAuthGrantMain } from '@/api/types'
import { formatDate } from '@/utils/format'

/**
 * Apps connected to the MCP server over OAuth (Claude connectors, Claude
 * Code) by the current user. A connection acts like an MCP-only API key of
 * the user; disconnecting stops its tokens at once.
 */

const message = useMessage()

const grants = ref<OAuthGrantMain[]>([])
const loading = ref(true)
const removing = ref('')

function formatUnix(value: number): string {
  return value ? formatDate(new Date(value * 1000).toISOString()) : 'Never'
}

async function load(): Promise<void> {
  loading.value = true
  try {
    grants.value = (await listOAuthGrants()).results
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to load connected apps'))
  } finally {
    loading.value = false
  }
}

async function disconnect(grant: OAuthGrantMain): Promise<void> {
  removing.value = grant.id
  try {
    await deleteOAuthGrant(grant.id)
    grants.value = grants.value.filter((item) => item.id !== grant.id)
    message.success(`${grant.client_name} disconnected`)
  } catch (error) {
    message.error(apiErrorMessage(error, 'Failed to disconnect'))
  } finally {
    removing.value = ''
  }
}

onMounted(load)
</script>

<template>
  <NCard title="Connected apps">
    <NText depth="3" class="grant-hint">
      MCP clients you allowed to sign in with your account (Claude connectors, Claude
      Code). Each one works like your MCP-only API key.
    </NText>

    <NSpin v-if="loading" size="small" />
    <NEmpty v-else-if="!grants.length" description="No connected apps" size="small" />

    <ul v-else class="grant-list">
      <li v-for="grant in grants" :key="grant.id" class="grant-row">
        <div class="grant-row__main">
          <div class="grant-row__name">{{ grant.client_name }}</div>
          <NText depth="3" class="grant-row__meta">
            {{ grant.redirect_host }} · connected
            {{ formatUnix(grant.connected_at_unix) }} · last used
            {{ formatUnix(grant.last_used_at_unix) }}
          </NText>
        </div>
        <NPopconfirm @positive-click="disconnect(grant)">
          <template #trigger>
            <NButton size="small" :loading="removing === grant.id">Disconnect</NButton>
          </template>
          Disconnect "{{ grant.client_name }}"? It loses access at once.
        </NPopconfirm>
      </li>
    </ul>
  </NCard>
</template>

<style scoped>
.grant-hint {
  display: block;
  margin-bottom: 12px;
  font-size: 13px;
}

.grant-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.grant-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.grant-row__main {
  min-width: 0;
}

.grant-row__name {
  font-weight: 500;
  overflow-wrap: anywhere;
}

.grant-row__meta {
  font-size: 12px;
  overflow-wrap: anywhere;
}
</style>
