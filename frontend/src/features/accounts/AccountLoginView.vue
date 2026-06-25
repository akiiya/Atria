<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMutation } from '@tanstack/vue-query'
import { apiPost } from '@/api/http'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const router = useRouter()
const step = ref<'phone' | 'code' | 'password'>('phone')
const phone = ref('')
const code = ref('')
const password = ref('')
const flowId = ref('')
const error = ref('')
const loading = ref(false)

const startMutation = useMutation({
  mutationFn: () => apiPost<any>('/api/accounts/login/start', { phone: phone.value }),
  onSuccess: (data) => {
    if (data.ok) {
      flowId.value = data.flow_id || ''
      step.value = 'code'
      error.value = ''
    } else {
      error.value = data.message || t('login.sendCodeFailed')
    }
    loading.value = false
  },
  onError: (err: Error) => {
    error.value = err.message || t('login.networkError')
    loading.value = false
  },
})

const codeMutation = useMutation({
  mutationFn: () => apiPost<any>('/api/accounts/login/code', { flow_id: flowId.value, code: code.value }),
  onSuccess: (data) => {
    if (data.ok) {
      if (data.next === 'password') {
        step.value = 'password'
        error.value = ''
      } else if (data.redirect) {
        router.push(data.redirect)
      }
    } else {
      error.value = data.message || t('login.codeError')
    }
    loading.value = false
  },
  onError: (err: Error) => {
    error.value = err.message || t('login.networkError')
    loading.value = false
  },
})

const passwordMutation = useMutation({
  mutationFn: () => apiPost<any>('/api/accounts/login/password', { flow_id: flowId.value, password: password.value }),
  onSuccess: (data) => {
    if (data.ok && data.redirect) {
      router.push(data.redirect)
    } else {
      error.value = data?.message || t('login.passwordError')
    }
    loading.value = false
  },
  onError: (err: Error) => {
    error.value = err.message || t('login.networkError')
    loading.value = false
  },
})

function submitPhone() {
  if (!phone.value.trim()) { error.value = t('login.phoneRequired'); return }
  loading.value = true
  error.value = ''
  startMutation.mutate()
}

function submitCode() {
  if (!code.value.trim()) { error.value = t('login.enterCode'); return }
  loading.value = true
  error.value = ''
  codeMutation.mutate()
}

function submitPassword() {
  if (!password.value) { error.value = t('login.enterPassword'); return }
  loading.value = true
  error.value = ''
  passwordMutation.mutate()
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t('login.title') }}</h1>
      <p class="page-desc">{{ t('login.desc') }}</p>
    </div>

    <div class="card" style="max-width:500px;">
      <div class="card-body">
        <div v-if="error" class="alert alert-error">{{ error }}</div>

        <!-- Step 1: Phone -->
        <div v-if="step === 'phone'">
          <div class="form-group">
            <label class="form-label">{{ t('login.phone') }}</label>
            <input v-model="phone" type="text" class="form-input" placeholder="+8613800138000"
                   @keydown.enter="submitPhone" :disabled="loading">
            <div class="form-hint">{{ t('login.phoneHint') }}</div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="submitPhone" :disabled="loading">
              {{ loading ? t('login.sending') : t('login.sendCode') }}
            </button>
            <a href="/accounts" class="btn btn-outline">{{ t('common.cancel') }}</a>
          </div>
        </div>

        <!-- Step 2: Code -->
        <div v-if="step === 'code'">
          <div class="form-group">
            <label class="form-label">{{ t('login.code') }}</label>
            <input v-model="code" type="text" class="form-input" :placeholder="t('login.enterCode')"
                   @keydown.enter="submitCode" :disabled="loading" autocomplete="one-time-code">
            <div class="form-hint">{{ t('login.codeHint') }}</div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="submitCode" :disabled="loading">
              {{ loading ? t('login.verifying') : t('login.submitCode') }}
            </button>
            <button class="btn btn-outline" @click="step='phone'; error=''">{{ t('login.restart') }}</button>
          </div>
        </div>

        <!-- Step 3: Password (2FA) -->
        <div v-if="step === 'password'">
          <div class="form-group">
            <label class="form-label">{{ t('login.tfaPassword') }}</label>
            <input v-model="password" type="password" class="form-input" :placeholder="t('login.enterPassword')"
                   @keydown.enter="submitPassword" :disabled="loading">
            <div class="form-hint">{{ t('login.tfaRequired') }}</div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="submitPassword" :disabled="loading">
              {{ loading ? t('login.verifying') : t('login.submitPassword') }}
            </button>
            <button class="btn btn-outline" @click="step='phone'; error=''">{{ t('login.restart') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
