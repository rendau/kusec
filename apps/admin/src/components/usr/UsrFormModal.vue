<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  NAlert,
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
import { useAuthStore } from '@/stores/auth'
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

const { options: appOptions, loading: appsLoading, search, ensure } = useAppOptions()

const authStore = useAuthStore()

/**
 * The admin is editing their own account. Their username and password cannot
 * be changed from here — the backend refuses it (`no_permission`): those go
 * through My profile, where the current password is asked. Both fields are
 * locked and left out of the request; the rest is edited as for anyone.
 */
const isSelf = computed(
  () =>
    props.user !== null &&
    authStore.profile !== null &&
    String(props.user.id) === String(authStore.profile.id),
)

interface FormModel {
  name: string
  username: string
  password: string
  is_admin: boolean
  active: boolean
  /** Non-admin sees every application; when off, access is `app_ids` only. */
  all_apps: boolean
  /** Apps the user may access while `all_apps` is off; empty = no access. */
  app_ids: string[]
}

const model = reactive<FormModel>({
  name: '',
  username: '',
  password: '',
  is_admin: false,
  active: true,
  all_apps: false,
  app_ids: [],
})

// The access choice applies to non-admins only: admins see everything.
const showAccess = computed(() => !model.is_admin)
// "All applications" is off and nothing is picked — the user will see nothing.
// Allowed (an account can be prepared before its apps exist) but called out.
const noAccess = computed(
  () => showAccess.value && !model.all_apps && model.app_ids.length === 0,
)

/**
 * Access part of the request. With "all applications" on, the picked list is
 * not sent: one stored source of truth, no stale list resurfacing later.
 */
function accessPayload(): { all_apps: boolean; app_ids: string[] } {
  return {
    all_apps: model.all_apps,
    app_ids: model.all_apps ? [] : model.app_ids,
  }
}

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
    // A new user starts with no access: the admin grants it explicitly.
    model.all_apps = user?.all_apps ?? false
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
      ...accessPayload(),
    }),
  update: (user) => {
    const update: UsrUpdateReq = {
      name: model.name,
      is_admin: model.is_admin,
      active: model.active,
      ...accessPayload(),
    }
    // Own username and password are not sent at all (see `isSelf`).
    if (!isSelf.value) {
      update.username = model.username
      // Only send the password when the admin actually typed a new one.
      if (model.password) update.password = model.password
    }
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
      : [
          {
            required: true,
            message: 'Password is required',
            trigger: ['blur', 'input'],
          },
        ]),
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
        <NSpace vertical :size="4" style="width: 100%">
          <NInput
            v-model:value="model.username"
            placeholder="Login username"
            :disabled="isSelf"
            :clearable="!isSelf"
            :input-props="{ autocomplete: 'off' }"
          />
          <NText v-if="isSelf" depth="3" style="font-size: 12px">
            Your own username is changed in
            <RouterLink class="inline-link" :to="{ name: 'profile' }" @click="close"
              >My profile</RouterLink
            >.
          </NText>
        </NSpace>
      </NFormItem>
      <!-- Own password: not editable here, so no input at all — just where to go. -->
      <NFormItem v-if="isSelf" label="Password">
        <NText depth="3" style="font-size: 12px">
          Your own password is changed in
          <RouterLink class="inline-link" :to="{ name: 'profile' }" @click="close"
            >My profile</RouterLink
          >.
        </NText>
      </NFormItem>
      <NFormItem
        v-else
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
        <NText v-if="!showAccess" depth="3" style="font-size: 12px">
          Administrators always have access to all applications.
        </NText>
        <NSpace v-else vertical :size="8" style="width: 100%">
          <NSpace align="center" :size="8" :wrap-item="false">
            <NSwitch v-model:value="model.all_apps" aria-label="All applications" />
            <span>All applications</span>
          </NSpace>
          <template v-if="!model.all_apps">
            <NSelect
              v-model:value="model.app_ids"
              :options="appOptions"
              :loading="appsLoading"
              multiple
              filterable
              clearable
              placeholder="Select applications"
            />
            <NAlert v-if="noAccess" type="warning" title="No access">
              This user will not have access to any application. Pick applications or
              turn on “All applications”.
            </NAlert>
          </template>
          <NText v-else depth="3" style="font-size: 12px">
            Includes applications created later.
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
