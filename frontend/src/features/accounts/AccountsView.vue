<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiGet } from '@/api/http'

const { data, isLoading, error } = useQuery({
  queryKey: ['accounts'],
  queryFn: () => apiGet<any>('/api/accounts'),
  retry: 1,
})

const accounts = computed(() => data.value?.accounts || [])
const hasApi = computed(() => data.value?.has_api_key ?? false)
</script>

<template>
  <div>
    <div class="page-header" style="display:flex;justify-content:space-between;align-items:flex-start;">
      <div>
        <h1 class="page-title">账号会话</h1>
        <p class="page-desc">管理已接入的 Telegram 账号</p>
      </div>
      <a v-if="hasApi" href="/accounts/login" class="btn btn-primary">接入账号</a>
    </div>

    <div v-if="!hasApi" class="card">
      <div class="card-body" style="text-align:center;padding:48px 24px;">
        <div style="font-size:48px;margin-bottom:16px;">🔑</div>
        <div style="font-size:18px;font-weight:600;margin-bottom:8px;">请先配置 Telegram API Key</div>
        <div style="color:var(--text-secondary);margin-bottom:16px;">Atria 需要一套 Telegram API Key 才能接入账号。</div>
        <a href="/settings" class="btn btn-primary">去配置 API Key</a>
      </div>
    </div>

    <div v-else-if="isLoading" class="card"><div class="card-body"><LoadingSkeleton /></div></div>
    <div v-else-if="error" class="alert alert-error">加载失败</div>

    <div v-else>
      <div v-if="accounts.length === 0" class="card">
        <div class="card-body" style="text-align:center;padding:48px 24px;">
          <div style="font-size:48px;margin-bottom:16px;">📱</div>
          <div style="font-size:18px;font-weight:600;margin-bottom:8px;">暂无账号</div>
          <div style="color:var(--text-secondary);">当前将使用默认 Telegram API Key 接入账号</div>
        </div>
      </div>

      <div v-else class="card">
        <div class="card-header"><h3 class="card-title">账号列表</h3></div>
        <div class="card-body" style="padding:0;">
          <table class="table">
            <thead>
              <tr>
                <th>显示名</th>
                <th>Username</th>
                <th>Session 状态</th>
                <th>最后同步</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="acc in accounts" :key="acc.id">
                <td><strong>{{ acc.display_name }}</strong></td>
                <td>{{ acc.username ? '@' + acc.username : '-' }}</td>
                <td>
                  <span :class="['badge', acc.session_status === 'active' ? 'badge-success' : 'badge-warning']">
                    {{ acc.session_status === 'active' ? '有效' : acc.session_status || '无' }}
                  </span>
                </td>
                <td style="color:var(--text-secondary);">{{ acc.last_sync || '-' }}</td>
                <td>
                  <div style="display:flex;gap:8px;">
                    <a :href="'/accounts/' + acc.id" class="btn btn-sm btn-outline">查看</a>
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
