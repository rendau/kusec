<script setup lang="ts">
import { computed, onMounted, reactive } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NText,
} from 'naive-ui'
import type { FormRules } from 'naive-ui'

import { createUser, updateUser } from '@/api/usr'
import type { UsrMain, UsrUpdateReq } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useEntityForm } from '@/composables/useEntityForm'

const props = defineProps<{
  show: boolean
  /** The user being edited, or `null` when creating a new one. */
  user: UsrMain | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  saved: []
}>()

const {
  options: appOptions,
  loading: appsLoading,
  search,
  ensure,
} = useAppOptions()

interface FormModel {
  name: string
  username: string
  password: string
  is_admin: boolean
  active: boolean
  /** Apps this user may access; empty = all applications (backend default). */
  app_ids: string[]
}

const model = reactive<FormModel>({
  name: '',
  username: '',
  password: '',
  is_admin: false,
  active: true,
  app_ids: [],
})

const { formRef, submitting, isEdit, submit } = useEntityForm<UsrMain>({
  show: () => props.show,
  entity: () => props.user,
  seed: async (user) => {
    model.name = user?.name ?? ''
    model.username = user?.username ?? ''
    model.password = ''
    model.is_admin = user?.is_admin ?? false
    model.active = user?.active ?? true
    model.app_ids = [...(user?.app_ids ?? [])]
    // Resolve labels for any already-assigned apps not yet in the options list.
    await Promise.all(model.app_ids.map((id) => ensure(id)))
  },
  create: () =>
    createUser({
      name: model.name,
      username: model.username,
      password: model.password,
      is_admin: model.is_admin,
      active: model.active,
      app_ids: model.app_ids,
    }),
  update: (user) => {
    const update: UsrUpdateReq = {
      name: model.name,
      username: model.username,
      is_admin: model.is_admin,
      active: model.active,
      app_ids: model.app_ids,
    }
    // Only send the password when the admin actually typed a new one.
    if (model.password) update.password = model.password
    return updateUser(user.id, update)
  },
  messages: { created: 'User created', updated: 'User updated' },
  onSaved: () => {
    emit('saved')
    close()
  },
})

onMounted(() => {
  void search()
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: 'Name is required', trigger: ['blur', 'input'] }],
  username: [
    { required: true, message: 'Username is required', trigger: ['blur', 'input'] },
  ],
  // Password is mandatory only when creating a new user.
  password: isEdit.value
    ? []
    : [{ required: true, message: 'Password is required', trigger: ['blur', 'input'] }],
}))

function close(): void {
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit ? 'Edit user' : 'New user'"
    style="max-width: 520px"
    :mask-closable="!submitting"
    @update:show="emit('update:show', $event)"
  >
    <NForm
      ref="formRef"
      :model="model"
      :rules="rules"
      label-placement="top"
      :disabled="submitting"
    >
      <NFormItem label="Name" path="name">
        <NInput v-model:value="model.name" placeholder="Full name" clearable />
      </NFormItem>
      <NFormItem label="Username" path="username">
        <NInput v-model:value="model.username" placeholder="Login username" clearable />
      </NFormItem>
      <NFormItem
        :label="isEdit ? 'New password (leave blank to keep)' : 'Password'"
        path="password"
      >
        <NInput
          v-model:value="model.password"
          type="password"
          show-password-on="click"
          :placeholder="isEdit ? 'Unchanged' : 'Password'"
        />
      </NFormItem>
      <NFormItem label="Administrator" path="is_admin">
        <NSwitch v-model:value="model.is_admin" />
      </NFormItem>
      <NFormItem label="Application access" path="app_ids">
        <NSpace vertical :size="4" style="width: 100%">
          <NSelect
            v-model:value="model.app_ids"
            :options="appOptions"
            :loading="appsLoading"
            :disabled="model.is_admin"
            multiple
            filterable
            remote
            clearable
            :placeholder="
              model.is_admin ? 'All applications' : 'All applications (leave empty)'
            "
            @search="search"
          />
          <NText depth="3" style="font-size: 12px">
            {{
              model.is_admin
                ? 'Administrators always have access to all applications.'
                : 'Leave empty to grant access to all applications.'
            }}
          </NText>
        </NSpace>
      </NFormItem>
      <NFormItem label="Active" path="active">
        <NSwitch v-model:value="model.active" />
      </NFormItem>
    </NForm>

    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton type="primary" :loading="submitting" @click="submit">
          {{ isEdit ? 'Save' : 'Create' }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
