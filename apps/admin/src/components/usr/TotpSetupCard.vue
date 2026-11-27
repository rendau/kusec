<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NSpace,
  NText,
} from 'naive-ui'
import { Copy, DeviceMobile, Qrcode } from '@vicons/tabler'
import QRCode from 'qrcode'

import { ApiError, apiErrorMessage } from '@/api/http'
import { confirmTotp, enrollTotp } from '@/api/usr'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useClipboard } from '@/composables/useClipboard'

const emit = defineEmits<{
  /**
   * 2FA enabled. The backend has replaced the session with it: the new token
   * pair is already stored (see `confirmTotp`), the old one is dead.
   */
  confirmed: []
  /** The user backed out before enabling. */
  cancel: []
}>()

const { isMobile } = useBreakpoint()
const { copy } = useClipboard()

/**
 * Two steps: `password` — prove it is the account owner (the backend hands
 * out the key only against the current password), then `bind` — add the key
 * to the authenticator app and confirm with the first code.
 */
const step = ref<'password' | 'bind'>('password')

// ── Step 1: current password ───────────────────────────────
const password = ref('')
const enrolling = ref(false)
// Errors that are about the password itself are shown at its field…
const passwordError = ref('')
// …anything else (server down, no network) below the form.
const enrollError = ref('')

/** Codes of `POST /usr/totp/enroll` that belong to the password field. */
const PASSWORD_FIELD_CODES = new Set(['current_password_invalid', 'too_many_attempts'])

// Absent (not `undefined`) while there is no error — see ProfileView.
const passwordFeedback = computed(() =>
  passwordError.value
    ? { validationStatus: 'error' as const, feedback: passwordError.value }
    : {},
)

const secret = ref('')
const otpauthUrl = ref('')
const qrDataUrl = ref('')

// ── Step 2: first code ─────────────────────────────────────
const code = ref('')
const confirming = ref(false)
const confirmError = ref('')

async function submitPassword(): Promise<void> {
  if (enrolling.value) return
  passwordError.value = ''
  enrollError.value = ''
  if (!password.value) {
    passwordError.value = 'Enter your current password'
    return
  }

  enrolling.value = true
  try {
    const rep = await enrollTotp(password.value)
    const qr = await QRCode.toDataURL(rep.otpauth_url, { width: 220, margin: 1 })
    secret.value = rep.secret
    otpauthUrl.value = rep.otpauth_url
    qrDataUrl.value = qr
    // Not needed past this point — do not keep it in memory.
    password.value = ''
    step.value = 'bind'
  } catch (error) {
    const text = apiErrorMessage(error, 'Failed to start 2FA setup')
    if (error instanceof ApiError && PASSWORD_FIELD_CODES.has(error.code)) {
      passwordError.value = text
    } else {
      enrollError.value = text
    }
  } finally {
    enrolling.value = false
  }
}

async function submitConfirm(): Promise<void> {
  const trimmed = code.value.trim()
  if (!trimmed) {
    confirmError.value = 'Enter the 6-digit code'
    return
  }
  confirming.value = true
  confirmError.value = ''
  try {
    await confirmTotp(trimmed)
    emit('confirmed')
  } catch (error) {
    // A wrong code (`totp_invalid`) and a lockout (`too_many_attempts`) get
    // their own texts by code; the fallback is for everything else.
    confirmError.value = apiErrorMessage(error, 'Unable to confirm the code, try again')
    code.value = ''
  } finally {
    confirming.value = false
  }
}
</script>

<template>
  <div class="totp-setup">
    <!-- Step 1: the key is issued only against the current password. -->
    <NForm
      v-if="step === 'password'"
      label-placement="top"
      :disabled="enrolling"
      @submit.prevent="submitPassword"
    >
      <p class="totp-setup__hint">
        To set up two-factor authentication, confirm your current password first.
      </p>
      <NFormItem label="Current password" v-bind="passwordFeedback">
        <NInput
          v-model:value="password"
          type="password"
          show-password-on="click"
          placeholder="Current password"
          autofocus
          :input-props="{ autocomplete: 'current-password' }"
          @update:value="passwordError = ''"
        />
      </NFormItem>
      <NSpace :size="8">
        <NButton type="primary" attr-type="submit" :loading="enrolling">
          Continue
        </NButton>
        <NButton :disabled="enrolling" @click="emit('cancel')">Cancel</NButton>
      </NSpace>
      <NAlert
        v-if="enrollError"
        type="error"
        :show-icon="false"
        style="margin-top: 12px"
      >
        {{ enrollError }}
      </NAlert>
    </NForm>

    <!-- Step 2: bind the key and confirm with the first code. -->
    <template v-else>
      <!-- Mobile: the authenticator app lives on the same phone, so a QR is
           useless — offer a one-tap deep link plus the secret to copy. -->
      <template v-if="isMobile">
        <p class="totp-setup__hint">
          Open your authenticator app and add the key, or tap below to add it
          automatically.
        </p>
        <NButton
          tag="a"
          :href="otpauthUrl"
          type="primary"
          block
          class="totp-setup__deeplink"
        >
          <template #icon>
            <NIcon :component="DeviceMobile" />
          </template>
          Open in authenticator app
        </NButton>
      </template>

      <!-- Desktop: scan the QR with the phone's authenticator app. -->
      <template v-else>
        <p class="totp-setup__hint">
          Scan this QR code with your authenticator app (Google Authenticator, Authy,
          1Password…), then enter the 6-digit code below.
        </p>
        <div class="totp-setup__qr">
          <img :src="qrDataUrl" alt="TOTP QR code" width="220" height="220" />
        </div>
      </template>

      <!-- Manual entry fallback (both platforms). -->
      <div class="totp-setup__secret">
        <NText :depth="3" style="font-size: 12px">
          <NIcon :component="Qrcode" /> Or enter this key manually:
        </NText>
        <div class="totp-setup__secret-row">
          <NText code class="totp-setup__secret-value">{{ secret }}</NText>
          <NButton
            quaternary
            circle
            size="small"
            aria-label="Copy key"
            @click="copy(secret, 'Key copied')"
          >
            <template #icon>
              <NIcon :component="Copy" />
            </template>
          </NButton>
        </div>
      </div>

      <NInput
        v-model:value="code"
        class="totp-setup__code"
        placeholder="123456"
        maxlength="6"
        :input-props="{ inputmode: 'numeric', autocomplete: 'one-time-code' }"
        @keyup.enter="submitConfirm"
      />

      <NAlert
        v-if="confirmError"
        type="error"
        :show-icon="false"
        style="margin-top: 12px"
      >
        {{ confirmError }}
      </NAlert>

      <NButton
        type="primary"
        block
        :loading="confirming"
        class="totp-setup__submit"
        @click="submitConfirm"
      >
        Enable 2FA
      </NButton>
    </template>
  </div>
</template>

<style scoped>
.totp-setup__hint {
  margin: 0 0 16px;
  font-size: 13px;
  opacity: 0.8;
}

.totp-setup__deeplink {
  margin-bottom: 16px;
}

.totp-setup__qr {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
}

.totp-setup__qr img {
  border-radius: 8px;
  background: #fff;
  padding: 8px;
}

.totp-setup__secret {
  margin-bottom: 16px;
}

.totp-setup__secret-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}

.totp-setup__secret-value {
  font-size: 13px;
  word-break: break-all;
}

.totp-setup__code {
  margin-top: 4px;
}

.totp-setup__submit {
  margin-top: 16px;
}
</style>
