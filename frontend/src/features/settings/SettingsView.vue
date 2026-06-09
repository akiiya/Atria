<script setup lang="ts">
import { ref } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { apiGet, apiPost } from '@/api/http'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'

const queryClient = useQueryClient()

// Settings data
const { data: settings, isLoading } = useQuery({
  queryKey: ['settings'],
  queryFn: () => apiGet<any>('/api/settings'),
  retry: 1,
})

// API Key form
const apiKeyForm = ref({ display_name: '', api_id: '', api_hash: '' })
const apiKeySaving = ref(false)
const apiKeyMsg = ref('')

function saveApiKey() {
  apiKeySaving.value = true
  apiKeyMsg.value = ''
  apiPost<any>('/settings/api-key', {
    display_name: apiKeyForm.value.display_name,
    api_id: apiKeyForm.value.api_id,
    api_hash: apiKeyForm.value.api_hash,
  }).then(data => {
    apiKeyMsg.value = data.ok ? 'API Key 已保存' : (data.message || '保存失败')
    apiKeySaving.value = false
    if (data.ok) queryClient.invalidateQueries({ queryKey: ['settings'] })
  }).catch(() => { apiKeyMsg.value = '保存失败'; apiKeySaving.value = false })
}

// Proxy form
const proxyForm = ref({ proxy_type: 'none', proxy_host: '', proxy_port: '', proxy_username: '', proxy_password: '', proxy_timeout: '30' })
const proxySaving = ref(false)
const proxyMsg = ref('')

function saveProxy() {
  proxySaving.value = true
  proxyMsg.value = ''
  apiPost<any>('/settings/proxy', proxyForm.value).then(data => {
    proxyMsg.value = data.ok ? '代理配置已保存' : (data.message || '保存失败')
    proxySaving.value = false
  }).catch(() => { proxyMsg.value = '保存失败'; proxySaving.value = false })
}

// Password form
const pwdForm = ref({ current_password: '', new_password: '', confirm_new_password: '' })
const pwdSaving = ref(false)
const pwdMsg = ref('')

function savePassword() {
  if (pwdForm.value.new_password !== pwdForm.value.confirm_new_password) {
    pwdMsg.value = '两次输入的密码不一致'; return
  }
  pwdSaving.value = true
  pwdMsg.value = ''
  apiPost<any>('/settings/password', pwdForm.value).then(data => {
    pwdMsg.value = data.ok ? '密码已修改，请重新登录' : (data.message || '修改失败')
    pwdSaving.value = false
  }).catch(() => { pwdMsg.value = '修改失败'; pwdSaving.value = false })
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">系统设置</h1>
      <p class="page-desc">系统配置与管理</p>
    </div>

    <div v-if="isLoading"><LoadingSkeleton /></div>

    <div v-else>
      <!-- 管理员安全 -->
      <div class="card" style="margin-bottom:16px;" id="admin-security">
        <div class="card-header"><h3 class="card-title">🔐 管理员安全</h3></div>
        <div class="card-body">
          <div v-if="pwdMsg" :class="['alert', pwdMsg.includes('失败') ? 'alert-error' : 'alert-success']">{{ pwdMsg }}</div>
          <div class="form-group">
            <label class="form-label">当前密码</label>
            <input v-model="pwdForm.current_password" type="password" class="form-input" placeholder="请输入当前密码">
          </div>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
            <div class="form-group">
              <label class="form-label">新密码</label>
              <input v-model="pwdForm.new_password" type="password" class="form-input" placeholder="至少 10 个字符">
            </div>
            <div class="form-group">
              <label class="form-label">确认新密码</label>
              <input v-model="pwdForm.confirm_new_password" type="password" class="form-input" placeholder="再次输入新密码">
            </div>
          </div>
          <div class="form-actions">
            <button class="btn btn-primary" @click="savePassword" :disabled="pwdSaving">
              {{ pwdSaving ? '保存中...' : '修改密码' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Telegram API Key -->
      <div class="card" style="margin-bottom:16px;" id="telegram-api-key">
        <div class="card-header"><h3 class="card-title">🔑 Telegram API Key</h3></div>
        <div class="card-body">
          <div class="alert alert-info" style="margin-bottom:16px;">
            <strong>💡 说明：</strong>通常只需要配置一套 Telegram API Key。多个 Telegram 账号可以使用同一套 API Key 登录。
          </div>
          <div v-if="apiKeyMsg" :class="['alert', apiKeyMsg.includes('失败') ? 'alert-error' : 'alert-success']">{{ apiKeyMsg }}</div>

          <!-- 展示态 -->
          <div v-if="settings?.api_key">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px;">
              <div><span style="color:var(--text-secondary);">名称</span><br><strong>{{ settings.api_key.display_name }}</strong></div>
              <div><span style="color:var(--text-secondary);">API ID</span><br><code>{{ settings.api_key.api_id_masked }}</code></div>
              <div><span style="color:var(--text-secondary);">API Hash</span><br><code>{{ settings.api_key.api_hash_hint }}</code></div>
              <div><span style="color:var(--text-secondary);">状态</span><br><span class="badge badge-success">已启用</span></div>
            </div>
            <button class="btn btn-outline" @click="apiKeyForm = { display_name: settings.api_key.display_name, api_id: '', api_hash: '' }">修改配置</button>
          </div>

          <!-- 编辑态 -->
          <div v-else>
            <div class="form-group">
              <label class="form-label">自定义名称</label>
              <input v-model="apiKeyForm.display_name" type="text" class="form-input" placeholder="Default API">
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
              <div class="form-group">
                <label class="form-label">API ID</label>
                <input v-model="apiKeyForm.api_id" type="text" class="form-input" placeholder="12345678">
              </div>
              <div class="form-group">
                <label class="form-label">API Hash</label>
                <input v-model="apiKeyForm.api_hash" type="text" class="form-input" placeholder="留空表示不修改">
                <div class="form-hint">API Hash 会加密保存</div>
              </div>
            </div>
            <div class="form-actions">
              <button class="btn btn-primary" @click="saveApiKey" :disabled="apiKeySaving">
                {{ apiKeySaving ? '保存中...' : '保存 API Key 配置' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- API 网络代理 -->
      <div class="card" style="margin-bottom:16px;" id="api-proxy">
        <div class="card-header"><h3 class="card-title">🌐 API 网络代理</h3></div>
        <div class="card-body">
          <div class="alert alert-info" style="margin-bottom:16px;">
            <strong>💡 说明：</strong>代理配置仅用于 Telegram MTProto API 连接，不影响 Web 界面访问。
          </div>
          <div v-if="proxyMsg" :class="['alert', proxyMsg.includes('失败') ? 'alert-error' : 'alert-success']">{{ proxyMsg }}</div>

          <div class="form-group">
            <label class="form-label">代理类型</label>
            <select v-model="proxyForm.proxy_type" class="form-input">
              <option value="none">不使用代理</option>
              <option value="https">HTTPS 代理</option>
              <option value="socks5">SOCKS5 代理</option>
            </select>
          </div>

          <div v-if="proxyForm.proxy_type !== 'none'">
            <div style="display:grid;grid-template-columns:2fr 1fr;gap:16px;">
              <div class="form-group">
                <label class="form-label">主机地址</label>
                <input v-model="proxyForm.proxy_host" type="text" class="form-input" placeholder="127.0.0.1">
              </div>
              <div class="form-group">
                <label class="form-label">端口</label>
                <input v-model="proxyForm.proxy_port" type="number" class="form-input" placeholder="1080">
              </div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
              <div class="form-group">
                <label class="form-label">用户名 <span style="color:var(--text-tertiary);">可选</span></label>
                <input v-model="proxyForm.proxy_username" type="text" class="form-input" placeholder="留空表示无需认证">
              </div>
              <div class="form-group">
                <label class="form-label">密码 <span style="color:var(--text-tertiary);">可选</span></label>
                <input v-model="proxyForm.proxy_password" type="password" class="form-input" placeholder="留空表示无需认证">
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">超时（秒）</label>
              <input v-model="proxyForm.proxy_timeout" type="number" class="form-input" placeholder="30" style="max-width:200px;">
            </div>
          </div>

          <div class="form-actions">
            <button class="btn btn-primary" @click="saveProxy" :disabled="proxySaving">
              {{ proxySaving ? '保存中...' : '保存代理配置' }}
            </button>
          </div>
        </div>
      </div>

      <!-- 系统信息 -->
      <div class="card" id="system-info">
        <div class="card-header"><h3 class="card-title">ℹ️ 系统信息</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">版本</span><br><strong>{{ settings?.version || '-' }}</strong></div>
            <div><span style="color:var(--text-secondary);">数据库</span><br>{{ settings?.db_driver || '-' }}</div>
            <div><span style="color:var(--text-secondary);">数据目录</span><br><code>{{ settings?.data_dir || '-' }}</code></div>
            <div><span style="color:var(--text-secondary);">监听地址</span><br><code>{{ settings?.listen_addr || '-' }}</code></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
