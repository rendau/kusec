<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NAlert, NButton, NIcon, NInput, NSpin, NText } from 'naive-ui'
import { Copy, DeviceMobile, Qrcode } from '@vicons/tabler'
import QRCode from 'qrcode'

import { apiErrorMessage } from '@/api/http'
import { confirmTotp, enrollTotp } from '@/api/usr'
import { useBreakpoint } from '@/composables/useBreakpoint'
import { useClipboard } from '@/composables/useClipboard'

const props = withDefaults(
  defineProps<{
    /**
     * Enrol via a setup token (mandatory-admin flow, no session yet). Omit to
     * enrol the currently authenticated user (voluntary, from the profile).
     */
    setupToken?: string
  }>(),
  { setupToken: '' },
)

const emit = defineEmits<{
  /** 2FA enabled — a fresh session token pair has been persisted. */
  confirmed: []
  cancel: []
}>()

const { isMobile } = useBreakpoint()
const { copy } = useClipboard()

const loading = ref(true)
const secret = ref('')
const otpauthUrl = ref('')
const qrDataUrl = ref('')
const enrollError = ref('')

const code = ref('')
const confirming = ref(false)
const confirmError = ref('')

async function loadEnrollment(): Promise<void> {
  loading.value = true
  enrollError.value = ''
  try {
    const rep = await enrollTotp(props.setupToken)
    secret.value = rep.secret
    otpauthUrl.value = rep.otpauth_url
    qrDataUrl.value = await QRCode.toDataURL(rep.otpauth_url, {
      width: 220,
      margin: 1,
    })
  } catch (error) {
    enrollError.value = apiErrorMessage(error, 'Failed to start 2FA setup')
  } finally {
    loading.value = false
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
    await confirmTotp(trimmed, props.setupToken)
    emit('confirmed')
  } catch (error) {
    confirmError.value = apiErrorMessage(error, 'Invalid code, try again')
    code.value = ''
  } finally {
    confirming.value = false
  }
}

onMounted(loadEnrollment)
</script>

<template>
  <div class="totp-setup">
    <NSpin v-if="loading" />

    <div v-else-if="enrollError">
      <NAlert type="error" :show-icon="false">{{ enrollError }}</NAlert>
      <NButton size="small" style="margin-top: 12px" @click="loadEnrollment">
        Retry
      </NButton>
    </div>

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
