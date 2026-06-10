<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NDropdown,
  NEmpty,
  NInput,
  NScrollbar,
  NSpin,
  NTag,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import { storeToRefs } from 'pinia'

import { ApiError } from '@/api/http'
import { deleteApp } from '@/api/app'
import type { AppMain } from '@/api/types'
import { useAppsStore } from '@/stores/apps'

import AppFormModal from '@/components/app/AppFormModal.vue'

const route = useRoute()
const router = useRouter()
const dialog = useDialog()
const message = useMessage()

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

const itemMenu: DropdownOption[] = [
  { label: 'Edit', key: 'edit' },
  { label: 'Delete', key: 'delete' },
]

const showForm = ref(false)
const editing = ref<AppMain | null>(null)

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function select(app: AppMain): void {
  void router.push({ name: 'app-workspace', params: { id: app.id } })
}

function onItemAction(app: AppMain, key: string | number): void {
  if (key === 'edit') {
    editing.value = app
    showForm.value = true
  } else if (key === 'delete') {
    confirmDelete(app)
  }
}

function confirmDelete(app: AppMain): void {
  dialog.warning({
    title: 'Delete application',
    content: `Delete "${app.name}"? This also deletes all of its secrets and items.`,
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        await deleteApp(app.id)
        message.success('Application deleted')
        await appsStore.refresh()
        if (currentId.value === app.id) {
          void router.push({ name: 'app-home' })
        }
      } catch (error) {
        message.error(
          error instanceof ApiError ? error.message : 'Failed to delete application',
        )
      }
    },
  })
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
          <span class="app-nav__item-main">
            <span class="app-nav__name">{{ app.name }}</span>
            <NTag
              v-if="!app.active"
              size="tiny"
              :bordered="false"
              type="warning"
            >
              off
            </NTag>
          </span>
          <NDropdown
            trigger="click"
            :options="itemMenu"
            @select="(key: string | number) => onItemAction(app, key)"
          >
            <NButton
              text
              size="tiny"
              class="app-nav__more"
              @click.stop
            >
              ⋯
            </NButton>
          </NDropdown>
        </button>
      </NSpin>
    </NScrollbar>

    <AppFormModal v-model:show="showForm" :app="editing" @saved="onSaved" />
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
  justify-content: space-between;
  gap: 8px;
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

.app-nav__item-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.app-nav__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-nav__more {
  opacity: 0.5;
}

.app-nav__item:hover .app-nav__more {
  opacity: 1;
}
</style>
