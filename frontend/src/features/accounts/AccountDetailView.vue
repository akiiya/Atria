<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { apiGet } from '@/api/http'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const route = useRoute()
const accountId = route.params.id

const { data: account, isLoading, error } = useQuery({
  queryKey: ['account', accountId],
  queryFn: () => apiGet<any>(`/api/accounts/${accountId}`),
  retry: 1,
})
</script>

<template>
  <div>
    <div class="page-header">
      <div style="display:flex;align-items:center;gap:12px;">
        <a href="/accounts" class="btn btn-outline btn-sm">← {{ t('common.back') }}</a>
        <div>
          <h1 class="page-title">{{ account?.display_name || t('accountDetail.title') }}</h1>
          <p class="page-desc" v-if="account?.username">@{{ account.username }}</p>
        </div>
      </div>
    </div>

    <div v-if="isLoading"><LoadingSkeleton /></div>
    <div v-else-if="error" class="alert alert-error">{{ t('common.error') }}</div>
    <div v-else-if="account">
      <div class="card" style="margin-bottom:16px;">
        <div class="card-header"><h3 class="card-title">{{ t('accountDetail.basicInfo') }}</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">{{ t('accountDetail.displayName') }}</span><br><strong>{{ account.display_name }}</strong></div>
            <div><span style="color:var(--text-secondary);">Username</span><br>{{ account.username ? '@' + account.username : '-' }}</div>
            <div><span style="color:var(--text-secondary);">{{ t('accountDetail.userId') }}</span><br><code>{{ account.user_id }}</code></div>
            <div><span style="color:var(--text-secondary);">{{ t('settings.status') }}</span><br><span :class="['badge', account.status === 'active' ? 'badge-success' : 'badge-warning']">{{ account.status }}</span></div>
          </div>
        </div>
      </div>

      <div class="card" style="margin-bottom:16px;">
        <div class="card-header"><h3 class="card-title">{{ t('accountDetail.sessionInfo') }}</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">{{ t('accountDetail.sessionStatus') }}</span><br><span :class="['badge', account.session_status === 'active' ? 'badge-success' : 'badge-warning']">{{ account.session_status || t('accountDetail.none') }}</span></div>
            <div><span style="color:var(--text-secondary);">{{ t('accounts.lastSync') }}</span><br>{{ account.last_sync || '-' }}</div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header"><h3 class="card-title" style="color:var(--color-danger);">{{ t('accountDetail.dangerZone') }}</h3></div>
        <div class="card-body">
          <p style="color:var(--text-secondary);margin-bottom:16px;">{{ t('accountDetail.dangerDesc') }}</p>
          <div style="display:flex;gap:12px;">
            <button class="btn btn-outline" disabled :title="t('accountDetail.developing')">{{ t('accountDetail.remoteLogout') }}</button>
            <button class="btn btn-danger" disabled :title="t('accountDetail.developing')">{{ t('accountDetail.deleteSession') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
