<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputGroup,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NText,
  useMessage,
} from 'naive-ui'
import type { FormItemInst, FormItemRule, FormRules } from 'naive-ui'
import { Copy } from '@vicons/tabler'

import { createUser, updateUser } from '@/api/usr'
import type { UsrMain, UsrUpdateReq } from '@/api/types'
import { useAppOptions } from '@/composables/useAppOptions'
import { useClipboard } from '@/composables/useClipboard'
import { useEntityForm } from '@/composables/useEntityForm'
import { generatePassword, passwordComplexityError } from '@/utils/password'

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

const message = useMessage()
const { write } = useClipboard()

const passwordItemRef = ref<FormItemInst | null>(null)
// A generated password is shown in clear text: the admin has to see what is
// being handed over, and it is a throwaway value copied into a message anyway.
const passwordRevealed = ref(false)

const { formRef, submitting, isEdit, submit } = useEntityForm<UsrMain>({
  show: () => props.show,
  entity: () => props.user,
  seed: async (user) => {
    model.name = user?.name ?? ''
    model.username = user?.username ?? ''
    model.password = ''
    passwordRevealed.value = false
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
  // Password is mandatory only when creating a new user; complexity is checked
  // whenever a password is actually entered.
  password: [
    ...(isEdit.value
      ? []
      : [{ required: true, message: 'Password is required', trigger: ['blur', 'input'] }]),
    {
      trigger: ['blur', 'input'],
      validator: (_rule: FormItemRule, value: string) => {
        if (!value) return true
        const err = passwordComplexityError(value)
        return err ? new Error(err) : true
      },
    },
  ],
}))

function fillGeneratedPassword(): void {
  model.password = generatePassword()
  passwordRevealed.value = true
  // A programmatic change does not fire the field's `input` trigger, so a stale
  // error from the previous value would otherwise stay on screen.
  passwordItemRef.value?.restoreValidation()
}

function onPasswordInput(value: string): void {
  // Typing from scratch goes back to a masked field.
  if (!value) passwordRevealed.value = false
}

const saveLabel = computed(() => (isEdit.value ? 'Save' : 'Create'))

/** Site URL + username + password as one message, ready to paste into a chat. */
function credentialsMessage(): string {
  const siteUrl = new URL(import.meta.env.BASE_URL, window.location.origin).href
  return [
    `Kusec: ${siteUrl}`,
    `Username: ${model.username.trim()}`,
    `Password: ${model.password}`,
  ].join('\n')
}

// Whether the running save was started by "… & copy" — picks the button that spins.
const savingWithCopy = ref(false)

/**
 * Save and put the sign-in details on the clipboard in one click. The clipboard
 * is written before the request: once the modal closes the password is gone,
 * so a user is never saved with a password that could not be copied.
 */
async function submitAndCopy(): Promise<void> {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  if (!(await write(credentialsMessage()))) {
    message.error(
      `Clipboard unavailable, nothing saved. Use "${saveLabel.value}" and copy the password manually.`,
    )
    return
  }

  savingWithCopy.value = true
  try {
    if (await submit()) message.success('Sign-in details copied')
  } finally {
    savingWithCopy.value = false
  }
}

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
        <NInput
          v-model:value="model.username"
          placeholder="Login username"
          clearable
          :input-props="{ autocomplete: 'off' }"
        />
      </NFormItem>
      <NFormItem
        ref="passwordItemRef"
        :label="isEdit ? 'New password (leave blank to keep)' : 'Password'"
        path="password"
      >
        <NSpace vertical :size="8" style="width: 100%">
          <NInputGroup>
            <NInput
              v-model:value="model.password"
              :type="passwordRevealed ? 'text' : 'password'"
              show-password-on="click"
              :placeholder="isEdit ? 'Unchanged' : 'Password'"
              :input-props="{ autocomplete: 'new-password' }"
              @update:value="onPasswordInput"
            />
            <NButton @click="fillGeneratedPassword">Generate</NButton>
          </NInputGroup>
          <NText depth="3" style="font-size: 12px">
            Once a password is set, “{{ saveLabel }} &amp; copy” also copies the site
            URL, username and password as one message.
          </NText>
        </NSpace>
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
            clearable
            :placeholder="
              model.is_admin ? 'All applications' : 'All applications (leave empty)'
            "
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
        <NButton
          :type="model.password ? 'default' : 'primary'"
          :loading="submitting && !savingWithCopy"
          :disabled="savingWithCopy"
          @click="submit"
        >
          {{ saveLabel }}
        </NButton>
        <NButton
          v-if="model.password"
          type="primary"
          :loading="savingWithCopy"
          :disabled="submitting && !savingWithCopy"
          @click="submitAndCopy"
        >
          <template #icon>
            <NIcon :component="Copy" />
          </template>
          {{ saveLabel }} &amp; copy
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>
