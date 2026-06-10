<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NEmpty, NInput, NScrollbar, NSpin, NTag } from 'naive-ui'
import { storeToRefs } from 'pinia'

import type { AppMain } from '@/api/types'
import { useAppsStore } from '@/stores/apps'

import AppFormModal from '@/components/app/AppFormModal.vue'

const route = useRoute()
const router = useRouter()

const appsStore = useAppsStore()
const { apps, loading } = storeToRefs(appsStore)

const search = ref('')

const currentId = computed(() =>
  typeof route.params.id === 'string' ? route.params.id : null,
)

const filtered = computed<AppMain[]>(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return apps.value
  return apps.value.filter(
    (a) =>
      a.name.toLowerCase().includes(q) || a.namespace.toLowerCase().includes(q),
  )
})

const showForm = ref(false)

function openCreate(): void {
  showForm.value = true
}

function select(app: AppMain): void {
  void router.push({ name: 'app-workspace', params: { id: app.id } })
}

async function onSaved(): Promise<void> {
  await appsStore.refresh()
}

onMounted(() => {
  void appsStore.ensureLoaded()
})
</script>

<template>
  <div class="app-nav">
    <div class="app-nav__head">
      <span class="app-nav__title">Applications</span>
      <NButton size="tiny" type="primary" @click="openCreate">+ New</NButton>
    </div>

    <div class="app-nav__search">
      <NInput
        v-model:value="search"
        size="small"
        placeholder="Search applications"
        clearable
      />
    </div>

    <NScrollbar class="app-nav__list">
      <NSpin :show="loading">
        <div v-if="filtered.length === 0" class="app-nav__empty">
          <NEmpty size="small" description="No applications" />
        </div>
        <button
          v-for="app in filtered"
          :key="app.id"
          type="button"
          class="app-nav__item"
          :class="{ 'app-nav__item--active': app.id === currentId }"
          @click="select(app)"
        >
          <span class="app-nav__name">{{ app.name }}</span>
          <NTag
            v-if="!app.active"
            size="tiny"
            :bordered="false"
            type="warning"
          >
            off
          </NTag>
        </button>
      </NSpin>
    </NScrollbar>

    <AppFormModal v-model:show="showForm" :app="null" @saved="onSaved" />
  </div>
</template>

<style scoped>
.app-nav {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.app-nav__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px 8px;
}

.app-nav__title {
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.7;
}

.app-nav__search {
  padding: 0 12px 8px;
}

.app-nav__list {
  flex: 1;
  min-height: 0;
}

.app-nav__empty {
  padding: 24px 12px;
}

.app-nav__item {
  display: flex;
  align-items: center;
  gap: 6px;
  width: calc(100% - 16px);
  margin: 2px 8px;
  padding: 8px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.app-nav__item:hover {
  background: rgba(128, 128, 128, 0.12);
}

.app-nav__item--active {
  background: rgba(99, 226, 183, 0.16);
  font-weight: 600;
}

.app-nav__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
