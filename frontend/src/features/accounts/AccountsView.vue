<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiGet, apiPost } from '@/api/http'
import { useI18n } from '@/i18n'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

const { t } = useI18n()

interface AccountDTO {
  id: number
  display_name: string
  username: string
  user_id: number
  status: string
  session_status: string
  runtime_state: string
  last_error: string
  is_current_account: boolean
  last_sync: string
  updated_at: string
}

const { data, isLoading, error, refetch } = useQuery({
  queryKey: ['accounts'],
  queryFn: () => apiGet<{ ok: boolean; accounts: AccountDTO[]; has_api_key: boolean }>('/api/accounts'),
  retry: 1,
})

const accounts = computed(() => data.value?.accounts || [])
const hasApi = computed(() => data.value?.has_api_key ?? false)

// Action feedback
const actionMsg = ref('')
const actionLoading = ref(false)

async function selectAccount(id: number) {
  actionLoading.value = true
  actionMsg.value = ''
  try {
    const form = document.createElement('form')
    form.method = 'POST'
    form.action = '/accounts/select'
    form.style.display = 'none'
    const csrfInput = document.createElement('input')
    csrfInput.type = 'hidden'
    csrfInput.name = 'csrf_token'
    csrfInput.value = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || ''
    form.appendChild(csrfInput)
    const idInput = document.createElement('input')
    idInput.type = 'hidden'
    idInput.name = 'account_id'
    idInput.value = String(id)
    form.appendChild(idInput)
    document.body.appendChild(form)
    form.submit()
  } catch {
    actionMsg.value = t('common.error')
    actionLoading.value = false
  }
}

async function startRuntime(id: number) {
  actionLoading.value = true
  try {
    const result = await apiPost<{ ok: boolean; message?: string }>('/api/chats/runtime/start', { account_id: id })
    if (result.ok) {
      actionMsg.value = t('accounts.runtimeStarted')
      refetch()
    } else {
      actionMsg.value = result.message || t('common.error')
    }
  } catch {
    actionMsg.value = t('common.error')
  }
  actionLoading.value = false
}

async function stopRuntime(id: number) {
  actionLoading.value = true
  try {
    const result = await apiPost<{ ok: boolean; message?: string }>('/api/chats/runtime/stop', { account_id: id })
    if (result.ok) {
      actionMsg.value = t('accounts.runtimeStopped')
      refetch()
    } else {
      actionMsg.value = result.message || t('common.error')
    }
  } catch {
    actionMsg.value = t('common.error')
  }
  actionLoading.value = false
}

async function enableAccount(id: number) {
  actionLoading.value = true
  actionMsg.value = ''
  try {
    const result = await apiPost<{ ok: boolean; message?: string }>(`/api/accounts/${id}/enable`, {})
    if (result.ok) {
      actionMsg.value = t('accounts.enabled')
      refetch()
    } else {
      actionMsg.value = result.message || t('common.error')
    }
  } catch {
    actionMsg.value = t('common.error')
  }
  actionLoading.value = false
}

async function disableAccount(id: number) {
  if (!confirm(t('accounts.confirmDisable'))) return
  actionLoading.value = true
  actionMsg.value = ''
  try {
    const result = await apiPost<{ ok: boolean; message?: string }>(`/api/accounts/${id}/disable`, {})
    if (result.ok) {
      actionMsg.value = t('accounts.disabled')
      refetch()
    } else {
      actionMsg.value = result.message || t('common.error')
    }
  } catch {
    actionMsg.value = t('common.error')
  }
  actionLoading.value = false
}

function runtimeStateClass(state: string): string {
  if (state === 'live' || state === 'syncing') return 'badge-success'
  if (state === 'connecting') return 'badge-warning'
  if (state === 'degraded' || state === 'error') return 'badge-danger'
  return 'badge-info'
}

function runtimeStateLabel(state: string): string {
  const key = 'accounts.runtime.' + state
  const translated = t(key)
  return translated !== key ? translated : state
}

function sessionStatusLabel(status: string): string {
  if (status === 'active') return t('accounts.sessionActive')
  if (status === 'expired') return t('accounts.sessionExpired')
  if (status === 'invalid') return t('accounts.sessionInvalid')
  return status || t('accounts.sessionNone')
}

function accountStatusLabel(status: string): string {
  const key = 'accounts.status.' + status
  const translated = t(key)
  return translated !== key ? translated : status
}
</script>

<template>
  <div>
    <div class="page-header" style="display:flex;justify-content:space-between;align-items:flex-start;">
      <div>
        <h1 class="page-title">{{ t('accounts.title') }}</h1>
        <p class="page-desc">{{ t('accounts.desc') }}</p>
      </div>
      <div style="display:flex;gap:8px;">
        <button class="btn btn-sm btn-outline" @click="refetch()" :disabled="isLoading">{{ t('common.refresh') }}</button>
        <a v-if="hasApi" href="/accounts/login" class="btn btn-sm btn-primary">{{ t('accounts.addAccount') }}</a>
      </div>
    </div>

    <div v-if="!hasApi && !isLoading" class="card">
      <EmptyState
        icon="&#x1f511;"
        :title="t('accounts.noApiKey')"
        :description="t('accounts.noApiKeyDesc')"
      />
      <div style="text-align:center;padding:0 24px 24px;">
        <a href="/settings" class="btn btn-primary">{{ t('accounts.goToSettings') }}</a>
      </div>
    </div>

    <div v-else-if="isLoading"><LoadingSkeleton /></div>

    <div v-else-if="error">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>

    <div v-else-if="accounts.length === 0" class="card">
      <EmptyState
        icon="&#x1f4f1;"
        :title="t('accounts.noAccounts')"
        :description="t('accounts.noAccountsDesc')"
      />
    </div>

    <div v-else>
      <!-- Action feedback -->
      <div v-if="actionMsg" class="alert" :class="actionMsg.includes('Error') || actionMsg.includes('失败') ? 'alert-error' : 'alert-success'" style="margin-bottom:16px;">
        {{ actionMsg }}
        <button class="alert-dismiss" @click="actionMsg = ''">&times;</button>
      </div>

      <div class="card">
        <div class="card-header"><h3 class="card-title">{{ t('accounts.accountList') }}</h3></div>
        <div class="card-body" style="padding:0;">
          <table class="table">
            <thead>
              <tr>
                <th></th>
                <th>{{ t('accounts.displayName') }}</th>
                <th>Username</th>
                <th>{{ t('accounts.statusTitle') }}</th>
                <th>{{ t('accounts.sessionTitle') }}</th>
                <th>{{ t('accounts.runtimeTitle') }}</th>
                <th>{{ t('accounts.lastSync') }}</th>
                <th>{{ t('accounts.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="acc in accounts" :key="acc.id" :style="acc.is_current_account ? 'background:var(--accent-light,rgba(59,130,246,0.05));' : ''">
                <td style="width:32px;">
                  <span v-if="acc.is_current_account" :title="t('accounts.currentAccount')" style="color:var(--accent-color);font-size:16px;">&#x25cf;</span>
                </td>
                <td><strong>{{ acc.display_name }}</strong></td>
                <td style="color:var(--text-secondary);">{{ acc.username ? '@' + acc.username : '-' }}</td>
                <td>
                  <span :class="['badge', acc.status === 'active' ? 'badge-success' : acc.status === 'disabled' ? 'badge-info' : 'badge-warning']">
                    {{ accountStatusLabel(acc.status) }}
                  </span>
                </td>
                <td>
                  <span :class="['badge', acc.session_status === 'active' ? 'badge-success' : 'badge-warning']">
                    {{ sessionStatusLabel(acc.session_status) }}
                  </span>
                </td>
                <td>
                  <span :class="['badge', runtimeStateClass(acc.runtime_state)]">
                    {{ runtimeStateLabel(acc.runtime_state) }}
                  </span>
                  <div v-if="acc.last_error" style="font-size:11px;color:var(--color-danger);margin-top:2px;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" :title="acc.last_error">
                    {{ acc.last_error }}
                  </div>
                </td>
                <td style="color:var(--text-secondary);font-size:13px;">{{ acc.last_sync || '-' }}</td>
                <td>
                  <div style="display:flex;gap:4px;flex-wrap:wrap;">
                    <button v-if="!acc.is_current_account && acc.status === 'active'" class="btn btn-sm btn-outline" @click="selectAccount(acc.id)" :disabled="actionLoading">
                      {{ t('accounts.select') }}
                    </button>
                    <button v-if="acc.status === 'active' && (acc.runtime_state === 'stopped' || acc.runtime_state === 'offline')" class="btn btn-sm btn-primary" @click="startRuntime(acc.id)" :disabled="actionLoading">
                      {{ t('accounts.startRuntime') }}
                    </button>
                    <button v-if="acc.status === 'active' && (acc.runtime_state === 'live' || acc.runtime_state === 'syncing' || acc.runtime_state === 'connecting')" class="btn btn-sm btn-outline" @click="stopRuntime(acc.id)" :disabled="actionLoading">
                      {{ t('accounts.stopRuntime') }}
                    </button>
                    <button v-if="acc.status === 'active'" class="btn btn-sm btn-outline" @click="disableAccount(acc.id)" :disabled="actionLoading" style="color:var(--color-danger);">
                      {{ t('accounts.disable') }}
                    </button>
                    <button v-if="acc.status === 'disabled'" class="btn btn-sm btn-primary" @click="enableAccount(acc.id)" :disabled="actionLoading">
                      {{ t('accounts.enable') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
