<script setup lang="ts">
import { ref } from 'vue'
import { NCard, NTabPane, NTabs } from 'naive-ui'

import AuditSection from '@/components/audit/AuditSection.vue'
import SyncRunSection from '@/components/syncrun/SyncRunSection.vue'

// Global feed: the audit trail across all accessible apps plus the sync
// journal. Per-app views live as tabs inside the app workspace.
const activeTab = ref<'audit' | 'sync'>('audit')
</script>

<template>
  <NCard title="Audit">
    <NTabs v-model:value="activeTab" type="line" animated display-directive="show:lazy">
      <NTabPane name="audit" tab="Changes">
        <AuditSection />
      </NTabPane>
      <NTabPane name="sync" tab="Sync runs">
        <SyncRunSection />
      </NTabPane>
    </NTabs>
  </NCard>
</template>
