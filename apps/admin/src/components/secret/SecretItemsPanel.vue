<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NEllipsis,
  NFlex,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

import { ApiError } from '@/api/http'
import { deleteItem, listItems } from '@/api/item'
import type { ItemListReq, ItemMain } from '@/api/types'

import ItemDetailDrawer from '@/components/item/ItemDetailDrawer.vue'
import ItemFormModal from '@/components/item/ItemFormModal.vue'

const props = defineProps<{
  secretId: string
}>()

const message = useMessage()

const rows = ref<ItemMain[]>([])
const loading = ref(false)
const revealed = ref(new Set<string>())

const detailId = ref<string | null>(null)
const showDetail = ref(false)

const editing = ref<ItemMain | null>(null)
const showForm = ref(false)

async function fetchItems(): Promise<void> {
  loading.value = true
  try {
    // Scoped by secret_id → backend returns all items, no pagination needed.
    const req: ItemListReq = { secret_id: props.secretId }
    const rep = await listItems(req)
    rows.value = rep.results ?? []
    revealed.value = new Set()
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to load items')
  } finally {
    loading.value = false
  }
}

function toggleReveal(id: string): void {
  const next = new Set(revealed.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  revealed.value = next
}

function openDetail(row: ItemMain): void {
  detailId.value = row.id
  showDetail.value = true
}

function openCreate(): void {
  editing.value = null
  showForm.value = true
}

function openEdit(row: ItemMain): void {
  editing.value = row
  showForm.value = true
}

async function removeItem(row: ItemMain): Promise<void> {
  try {
    await deleteItem(row.id)
    message.success('Item deleted')
    await fetchItems()
  } catch (error) {
    message.error(error instanceof ApiError ? error.message : 'Failed to delete item')
  }
}

const columns: DataTableColumns<ItemMain> = [
  {
    title: 'Key',
    key: 'key',
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', onClick: () => openDetail(row) },
        { default: () => row.key },
      ),
  },
  {
    title: 'Value',
    key: 'value',
    render: (row) =>
      h(NSpace, { align: 'center', size: 8, wrapItem: false }, () => [
        revealed.value.has(row.id)
          ? h(
              NEllipsis,
              { style: 'max-width: 220px' },
              { default: () => row.value || '—' },
            )
          : h(NText, { depth: 3 }, { default: () => '••••••••' }),
        h(
          NButton,
          {
            text: true,
            type: 'primary',
            size: 'tiny',
            onClick: () => toggleReveal(row.id),
          },
          { default: () => (revealed.value.has(row.id) ? 'Hide' : 'Show') },
        ),
      ]),
  },
  {
    title: 'Status',
    key: 'active',
    width: 100,
    render: (row) =>
      h(
        NTag,
        { type: row.active ? 'success' : 'default', size: 'small' },
        { default: () => (row.active ? 'Active' : 'Inactive') },
      ),
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 200,
    render: (row) =>
      h(NSpace, { size: 8 }, () => [
        h(
          NButton,
          { size: 'tiny', onClick: () => openEdit(row) },
          { default: () => 'Edit' },
        ),
        h(
          NPopconfirm,
          { onPositiveClick: () => removeItem(row) },
          {
            trigger: () =>
              h(
                NButton,
                { size: 'tiny', type: 'error', secondary: true },
                { default: () => 'Delete' },
              ),
            default: () => `Delete "${row.key}"?`,
          },
        ),
      ]),
  },
]

onMounted(fetchItems)
</script>

<template>
  <div class="items-panel">
    <NFlex justify="space-between" align="center" style="margin-bottom: 8px">
      <NText depth="3" style="font-size: 12px">Items</NText>
      <NButton size="tiny" type="primary" @click="openCreate">New item</NButton>
    </NFlex>

    <NDataTable
      size="small"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(row: ItemMain) => row.id"
      :pagination="false"
    />

    <ItemDetailDrawer v-model:show="showDetail" :item-id="detailId" />
    <ItemFormModal
      v-model:show="showForm"
      :item="editing"
      :default-secret-id="secretId"
      lock-secret
      @saved="fetchItems"
    />
  </div>
</template>

<style scoped>
.items-panel {
  padding: 4px 8px 8px;
}
</style>
