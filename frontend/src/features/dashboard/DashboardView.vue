<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { useRouter } from 'vue-router'
import { fetchDashboardStats } from '@/api/me'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const router = useRouter()

const { data, isLoading, error, refetch } = useQuery({
  queryKey: ['dashboard-stats'],
  queryFn: fetchDashboardStats,
  refetchInterval: 30_000,
})

function riskBadgeClass(level: string): string {
  if (level === 'high' || level === 'critical') return 'badge-danger'
  if (level === 'medium') return 'badge-warning'
  return 'badge-success'
}

function formatTime(at: string): string {
  if (!at) return '-'
  try {
    const d = new Date(at.replace(' ', 'T') + 'Z')
    return d.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch {
    return at
  }
}

function actionLabel(action: string): string {
  return t('event.' + action) || action
}
</script>

<template>
  <div>
    <div class="page-header" style="display:flex;justify-content:space-between;align-items:flex-start;">
      <div>
        <h1 class="page-title">{{ t('dashboard.title') }}</h1>
        <p class="page-desc">{{ t('dashboard.desc') }}</p>
      </div>
      <button class="btn btn-sm btn-outline" @click="refetch()">{{ t('common.refresh') }}</button>
    </div>

    <div v-if="isLoading" class="stats-grid">
      <div v-for="i in 6" :key="i" class="card stat-card">
        <div class="skeleton-line long" style="height:32px;width:48px;margin:0 auto 8px"></div>
        <div class="skeleton-line short" style="height:14px;width:60px;margin:0 auto"></div>
      </div>
    </div>

    <div v-else-if="error" class="alert alert-error">{{ t('common.error') }}</div>

    <div v-else>
      <!-- Stats grid -->
      <div class="stats-grid">
        <div class="card stat-card">
          <div class="stat-icon">📱</div>
          <div class="stat-value">{{ data?.account_count ?? 0 }}</div>
          <div class="stat-label">{{ t('dashboard.accounts') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">🟢</div>
          <div class="stat-value">{{ data?.runtime_live ?? 0 }}</div>
          <div class="stat-label">{{ t('chat.runtimeLive') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">🔴</div>
          <div class="stat-value">{{ data?.runtime_stopped ?? 0 }}</div>
          <div class="stat-label">{{ t('chat.runtimeStopped') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">🔑</div>
          <div class="stat-value">{{ data?.api_key_count ?? 0 }}</div>
          <div class="stat-label">{{ t('dashboard.apiKeys') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">⚠️</div>
          <div class="stat-value">{{ data?.recent_errors ?? 0 }}</div>
          <div class="stat-label">{{ t('dashboard.recentErrors') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">📋</div>
          <div class="stat-value">{{ data?.recent_audit ?? 0 }}</div>
          <div class="stat-label">{{ t('dashboard.auditToday') }}</div>
        </div>
      </div>

      <!-- Quick links + Recent events -->
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:24px;">
        <!-- Quick links -->
        <div class="card">
          <div class="card-header"><h3 class="card-title">{{ t('dashboard.quickLinks') }}</h3></div>
          <div class="card-body" style="display:flex;flex-direction:column;gap:8px;">
            <a href="#" @click.prevent="router.push('/chats')" class="dropdown-item">
              <span>💬</span> {{ t('nav.chats') }}
            </a>
            <a href="#" @click.prevent="router.push('/contacts')" class="dropdown-item">
              <span>👥</span> {{ t('nav.contacts') }}
            </a>
            <a href="#" @click.prevent="router.push('/accounts')" class="dropdown-item">
              <span>📱</span> {{ t('nav.accounts') }}
            </a>
            <a href="#" @click.prevent="router.push('/settings')" class="dropdown-item">
              <span>⚙️</span> {{ t('nav.settings') }}
            </a>
            <a href="#" @click.prevent="router.push('/audit')" class="dropdown-item">
              <span>📋</span> {{ t('nav.audit') }}
            </a>
          </div>
        </div>

        <!-- Recent audit events -->
        <div class="card">
          <div class="card-header"><h3 class="card-title">{{ t('dashboard.recentAudit') }}</h3></div>
          <div class="card-body" style="padding:0;">
            <div v-if="!data?.recent_logs?.length" style="text-align:center;padding:24px;color:var(--text-secondary);">
              {{ t('audit.empty') }}
            </div>
            <table v-else class="table">
              <tbody>
                <tr v-for="log in data.recent_logs" :key="log.id">
                  <td style="white-space:nowrap;color:var(--text-secondary);font-size:12px;">{{ formatTime(log.created_at) }}</td>
                  <td style="font-size:13px;">{{ actionLabel(log.action) }}</td>
                  <td>
                    <span :class="['badge', riskBadgeClass(log.risk_level)]">{{ log.risk_level }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- System info -->
      <div class="card" style="margin-top:16px;">
        <div class="card-header"><h3 class="card-title">{{ t('dashboard.systemInfo') }}</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.version') }}</span><br><strong>{{ data?.version || 'dev' }}</strong></div>
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.database') }}</span><br><strong>{{ data?.db_driver || 'sqlite' }}</strong></div>
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.dataDir') }}</span><br><code>{{ data?.data_dir || './data' }}</code></div>
            <div><span style="color:var(--text-secondary);">{{ t('dashboard.listenAddr') }}</span><br><code>{{ data?.listen_addr || '127.0.0.1:8080' }}</code></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
