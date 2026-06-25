<script setup lang="ts">
import { ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { apiGet, apiPost } from '@/api/http'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const queryClient = useQueryClient()

// Settings data
const { data: settings, isLoading } = useQuery({
  queryKey: ['settings'],
  queryFn: () => apiGet<any>('/api/settings'),
  retry: 1,
})

// API Key edit mode
const apiKeyEditMode = ref(false)
const apiKeyForm = ref({ display_name: '', api_id: '', api_hash: '' })
const apiKeySaving = ref(false)
const apiKeyMsg = ref('')
const apiKeyIsError = ref(false)

function enterApiKeyEdit() {
  apiKeyEditMode.value = true
  apiKeyForm.value = {
    display_name: settings.value?.api_key?.display_name || '',
    api_id: '',
    api_hash: '',
  }
}

function cancelApiKeyEdit() {
  apiKeyEditMode.value = false
  apiKeyMsg.value = ''
}

function saveApiKey() {
  apiKeySaving.value = true
  apiKeyMsg.value = ''
  apiPost<any>('/api/settings/api-key', {
    display_name: apiKeyForm.value.display_name,
    api_id: apiKeyForm.value.api_id,
    api_hash: apiKeyForm.value.api_hash,
  }).then(data => {
    if (data.ok) {
      apiKeyMsg.value = t('settings.apiKeySaved')
      apiKeyIsError.value = false
      apiKeyEditMode.value = false
      queryClient.invalidateQueries({ queryKey: ['settings'] })
    } else {
      apiKeyMsg.value = data.message || t('settings.saveFailed')
      apiKeyIsError.value = true
    }
    apiKeySaving.value = false
  }).catch(() => { apiKeyMsg.value = t('settings.saveFailed'); apiKeyIsError.value = true; apiKeySaving.value = false })
}

// Proxy form - initialized from settings data via watcher
const proxyForm = ref({
  proxy_type: 'none',
  proxy_host: '',
  proxy_port: '',
  proxy_username: '',
  proxy_password: '',
  proxy_timeout: '30',
  proxy_remark: '',
})
const proxySaving = ref(false)
const proxyMsg = ref('')
const proxyIsError = ref(false)
const proxyWarning = ref('')
const proxyLegacyInvalid = ref(false)
const proxyLegacyMessage = ref('')

// Sync proxy form from settings data when it loads
watch(settings, (val) => {
  if (val?.proxy) {
    // Determine proxy type: if enabled and type is set, use it; otherwise 'none'
    const isEnabled = val.proxy.enabled === 'true'
    const proxyType = val.proxy.type || 'none'
    proxyForm.value.proxy_type = (isEnabled && proxyType !== 'none') ? proxyType : 'none'
    proxyForm.value.proxy_host = val.proxy.host || ''
    proxyForm.value.proxy_port = val.proxy.port || ''
    proxyForm.value.proxy_username = val.proxy.username || ''
    proxyForm.value.proxy_timeout = val.proxy.timeout || '30'
    proxyForm.value.proxy_remark = val.proxy.remark || ''
    // 检测 legacy api_proxy 配置
    if (val.proxy.type === 'api_proxy') {
      proxyLegacyInvalid.value = true
      proxyLegacyMessage.value = val.proxy.legacy_message || t('settings.proxyRemoved')
    } else {
      proxyLegacyInvalid.value = false
      proxyLegacyMessage.value = ''
    }
    // Password is never returned from API, keep empty
    proxyForm.value.proxy_password = ''
  }
}, { immediate: true })

function saveProxy() {
  // 前端拦截：不允许保存 api_proxy
  if (proxyForm.value.proxy_type === 'api_proxy') {
    proxyMsg.value = t('settings.proxySelectHint')
    proxyIsError.value = false
    return
  }

  proxySaving.value = true
  proxyMsg.value = ''
  proxyWarning.value = ''
  apiPost<any>('/api/settings/proxy', proxyForm.value).then(data => {
    if (data.ok) {
      proxyMsg.value = t('settings.proxySaved')
      proxyIsError.value = false
      // 显示后端返回的 warning
      if (data.warning) {
        proxyWarning.value = data.warning
      }
      // 保存成功后清除 legacy 状态
      proxyLegacyInvalid.value = false
      proxyLegacyMessage.value = ''
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['runtime-status'] })
    } else {
      proxyMsg.value = data.message || t('settings.saveFailed')
      proxyIsError.value = true
    }
    proxySaving.value = false
  }).catch(() => { proxyMsg.value = t('settings.saveFailed'); proxyIsError.value = true; proxySaving.value = false })
}

// Password form
const pwdForm = ref({ current_password: '', new_password: '', confirm_new_password: '' })
const pwdSaving = ref(false)
const pwdMsg = ref('')
const pwdIsError = ref(false)

function savePassword() {
  if (pwdForm.value.new_password !== pwdForm.value.confirm_new_password) {
    pwdMsg.value = t('settings.passwordMismatch')
    pwdIsError.value = true
    return
  }
  pwdSaving.value = true
  pwdMsg.value = ''
  apiPost<any>('/settings/password', pwdForm.value).then(data => {
    if (data.ok) {
      pwdMsg.value = t('settings.passwordChanged')
      pwdIsError.value = false
    } else {
      pwdMsg.value = data.message || t('settings.changeFailed')
      pwdIsError.value = true
    }
    pwdSaving.value = false
  }).catch(() => { pwdMsg.value = t('settings.changeFailed'); pwdIsError.value = true; pwdSaving.value = false })
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t('settings.title') }}</h1>
      <p class="page-desc">{{ t('settings.desc') }}</p>
    </div>

    <div v-if="isLoading"><LoadingSkeleton /></div>

    <div v-else>
      <!-- 管理员安全 -->
      <div class="card" style="margin-bottom:16px;" id="admin-security">
        <div class="card-header"><h3 class="card-title">🔐 {{ t('settings.adminSecurity') }}</h3></div>
        <div class="card-body">
          <div v-if="pwdMsg" :class="['alert', pwdIsError ? 'alert-error' : 'alert-success']">{{ pwdMsg }}</div>
          <div class="form-group">
            <label class="form-label">{{ t('settings.currentPassword') }}</label>
            <input v-model="pwdForm.current_password" type="password" class="form-input" :placeholder="t('settings.enterCurrentPassword')">
          </div>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
            <div class="form-group">
              <label class="form-label">{{ t('settings.newPassword') }}</label>
              <input v-model="pwdForm.new_password" type="password" class="form-input" :placeholder="t('settings.minChars')">
            </div>
            <div class="form-group">
              <label class="form-label">{{ t('settings.confirmPassword') }}</label>
              <input v-model="pwdForm.confirm_new_password" type="password" class="form-input" :placeholder="t('settings.retypePassword')">
            </div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="savePassword" :disabled="pwdSaving">
              {{ pwdSaving ? t('settings.saving') : t('settings.changePassword') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Telegram API Key -->
      <div class="card" style="margin-bottom:16px;" id="telegram-api-key">
        <div class="card-header"><h3 class="card-title">🔑 Telegram API Key</h3></div>
        <div class="card-body">
          <div class="alert alert-info" style="margin-bottom:16px;">
            <strong>💡 {{ t('settings.apiKeyDescTitle') }}</strong>{{ t('settings.apiKeyDesc') }}
          </div>
          <div v-if="apiKeyMsg" :class="['alert', apiKeyIsError ? 'alert-error' : 'alert-success']">{{ apiKeyMsg }}</div>

          <!-- 展示态 -->
          <div v-if="settings?.api_key && !apiKeyEditMode">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px;">
              <div><span style="color:var(--text-secondary);">{{ t('settings.name') }}</span><br><strong>{{ settings.api_key.display_name }}</strong></div>
              <div><span style="color:var(--text-secondary);">API ID</span><br><code>{{ settings.api_key.api_id_masked }}</code></div>
              <div><span style="color:var(--text-secondary);">API Hash</span><br><code>{{ settings.api_key.api_hash_hint }}</code></div>
              <div><span style="color:var(--text-secondary);">{{ t('settings.status') }}</span><br><span class="badge badge-success">{{ t('settings.enabled') }}</span></div>
            </div>
            <button class="btn btn-outline" @click="enterApiKeyEdit()">{{ t('settings.editConfig') }}</button>
          </div>

          <!-- 编辑态 -->
          <div v-else>
            <div class="form-group">
              <label class="form-label">{{ t('settings.customName') }}</label>
              <input v-model="apiKeyForm.display_name" type="text" class="form-input" placeholder="Default API">
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
              <div class="form-group">
                <label class="form-label">API ID</label>
                <input v-model="apiKeyForm.api_id" type="text" class="form-input" placeholder="12345678">
              </div>
              <div class="form-group">
                <label class="form-label">API Hash</label>
                <input v-model="apiKeyForm.api_hash" type="text" class="form-input" :placeholder="t('settings.leaveEmpty')">
                <div class="form-hint">{{ t('settings.apiHashEncrypted') }}</div>
              </div>
            </div>
            <div class="form-actions">
              <button class="btn btn-primary" @click="saveApiKey" :disabled="apiKeySaving">
                {{ apiKeySaving ? t('settings.saving') : t('settings.saveApiKey') }}
              </button>
              <button v-if="settings?.api_key" class="btn btn-outline" @click="cancelApiKeyEdit()" :disabled="apiKeySaving">
                {{ t('common.cancel') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- 网络代理 -->
      <div class="card" style="margin-bottom:16px;" id="api-proxy">
        <div class="card-header"><h3 class="card-title">🌐 {{ t('settings.proxy') }}</h3></div>
        <div class="card-body">
          <div class="alert alert-info" style="margin-bottom:16px;">
            <strong>💡 {{ t('settings.proxyDescTitle') }}</strong>{{ t('settings.proxyDesc') }}
          </div>
          <div v-if="proxyLegacyInvalid" class="alert alert-warning" style="margin-bottom:16px;">
            <strong>⚠️ {{ t('settings.proxyInvalid') }}</strong>{{ proxyLegacyMessage || t('settings.proxyRemoved') }}<br>
            {{ t('settings.proxySelectHint') }}
          </div>
          <div v-if="proxyMsg" :class="['alert', proxyIsError ? 'alert-error' : 'alert-success']">{{ proxyMsg }}</div>
          <div v-if="proxyWarning" class="alert alert-warning" style="margin-bottom:16px;">{{ proxyWarning }}</div>

          <div class="form-group">
            <label class="form-label">{{ t('settings.proxyType') }}</label>
            <select v-model="proxyForm.proxy_type" class="form-input">
              <option value="none">{{ t('settings.proxyNone') }}</option>
              <option value="https">{{ t('settings.proxyHttps') }}</option>
              <option value="socks5">{{ t('settings.proxySocks5') }}</option>
            </select>
          </div>

          <!-- SOCKS5/HTTPS 模式 -->
          <div v-if="proxyForm.proxy_type === 'socks5' || proxyForm.proxy_type === 'https'">
            <div style="display:grid;grid-template-columns:2fr 1fr;gap:16px;">
              <div class="form-group">
                <label class="form-label">{{ t('settings.proxyHost') }}</label>
                <input v-model="proxyForm.proxy_host" type="text" class="form-input" placeholder="127.0.0.1">
              </div>
              <div class="form-group">
                <label class="form-label">{{ t('settings.proxyPort') }}</label>
                <input v-model="proxyForm.proxy_port" type="number" class="form-input" placeholder="1080">
              </div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
              <div class="form-group">
                <label class="form-label">{{ t('settings.proxyUsername') }} <span style="color:var(--text-tertiary);">{{ t('settings.proxyOptional') }}</span></label>
                <input v-model="proxyForm.proxy_username" type="text" class="form-input" :placeholder="t('settings.proxyNoAuth')">
              </div>
              <div class="form-group">
                <label class="form-label">{{ t('settings.proxyPassword') }} <span style="color:var(--text-tertiary);">{{ t('settings.proxyOptional') }}</span></label>
                <input v-model="proxyForm.proxy_password" type="password" class="form-input" :placeholder="t('settings.proxyNoAuth')">
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">{{ t('settings.proxyTimeout') }}</label>
              <input v-model="proxyForm.proxy_timeout" type="number" class="form-input" placeholder="30" style="max-width:200px;">
            </div>
          </div>

          <div class="form-actions">
            <button class="btn btn-primary" @click="saveProxy" :disabled="proxySaving">
              {{ proxySaving ? t('settings.saving') : t('settings.saveProxy') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 系统信息 -->
      <div class="card" id="system-info">
        <div class="card-header"><h3 class="card-title">ℹ️ {{ t('dashboard.systemInfo') }}</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.version') }}</span><br><strong>{{ settings?.version || '-' }}</strong></div>
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.database') }}</span><br>{{ settings?.db_driver || '-' }}</div>
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.dataDir') }}</span><br><code>{{ settings?.data_dir || '-' }}</code></div>
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.listenAddr') }}</span><br><code>{{ settings?.listen_addr || '-' }}</code></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
