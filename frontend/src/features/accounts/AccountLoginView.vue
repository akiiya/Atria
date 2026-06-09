<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMutation } from '@tanstack/vue-query'
import { apiPost } from '@/api/http'

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
      error.value = data.message || '发送验证码失败'
    }
    loading.value = false
  },
  onError: (err: Error) => {
    error.value = err.message || '网络请求失败'
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
      error.value = data.message || '验证码错误'
    }
    loading.value = false
  },
  onError: (err: Error) => {
    error.value = err.message || '网络请求失败'
    loading.value = false
  },
})

const passwordMutation = useMutation({
  mutationFn: () => apiPost<any>('/api/accounts/login/password', { flow_id: flowId.value, password: password.value }),
  onSuccess: (data) => {
    if (data.ok && data.redirect) {
      router.push(data.redirect)
    } else {
      error.value = data?.message || '密码错误'
    }
    loading.value = false
  },
  onError: (err: Error) => {
    error.value = err.message || '网络请求失败'
    loading.value = false
  },
})

function submitPhone() {
  if (!phone.value.trim()) { error.value = '请输入手机号'; return }
  loading.value = true
  error.value = ''
  startMutation.mutate()
}

function submitCode() {
  if (!code.value.trim()) { error.value = '请输入验证码'; return }
  loading.value = true
  error.value = ''
  codeMutation.mutate()
}

function submitPassword() {
  if (!password.value) { error.value = '请输入密码'; return }
  loading.value = true
  error.value = ''
  passwordMutation.mutate()
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">接入账号</h1>
      <p class="page-desc">通过手机号登录 Telegram 账号</p>
    </div>

    <div class="card" style="max-width:500px;">
      <div class="card-body">
        <div v-if="error" class="alert alert-error">{{ error }}</div>

        <!-- Step 1: Phone -->
        <div v-if="step === 'phone'">
          <div class="form-group">
            <label class="form-label">手机号</label>
            <input v-model="phone" type="text" class="form-input" placeholder="+8613800138000"
                   @keydown.enter="submitPhone" :disabled="loading">
            <div class="form-hint">请输入完整的国际手机号，以 + 开头</div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="submitPhone" :disabled="loading">
              {{ loading ? '发送中...' : '发送验证码' }}
            </button>
            <a href="/accounts" class="btn btn-outline">取消</a>
          </div>
        </div>

        <!-- Step 2: Code -->
        <div v-if="step === 'code'">
          <div class="form-group">
            <label class="form-label">验证码</label>
            <input v-model="code" type="text" class="form-input" placeholder="请输入验证码"
                   @keydown.enter="submitCode" :disabled="loading" autocomplete="one-time-code">
            <div class="form-hint">请填写 Telegram 发送的验证码</div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="submitCode" :disabled="loading">
              {{ loading ? '验证中...' : '提交验证码' }}
            </button>
            <button class="btn btn-outline" @click="step='phone'; error=''">重新开始</button>
          </div>
        </div>

        <!-- Step 3: Password (2FA) -->
        <div v-if="step === 'password'">
          <div class="form-group">
            <label class="form-label">两步验证密码</label>
            <input v-model="password" type="password" class="form-input" placeholder="请输入密码"
                   @keydown.enter="submitPassword" :disabled="loading">
            <div class="form-hint">该账号已开启两步验证</div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="submitPassword" :disabled="loading">
              {{ loading ? '验证中...' : '提交密码' }}
            </button>
            <button class="btn btn-outline" @click="step='phone'; error=''">重新开始</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
