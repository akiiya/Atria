<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { apiGet } from '@/api/http'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'

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
        <a href="/accounts" class="btn btn-outline btn-sm">← 返回</a>
        <div>
          <h1 class="page-title">{{ account?.display_name || '账号详情' }}</h1>
          <p class="page-desc" v-if="account?.username">@{{ account.username }}</p>
        </div>
      </div>
    </div>

    <div v-if="isLoading"><LoadingSkeleton /></div>
    <div v-else-if="error" class="alert alert-error">加载失败</div>
    <div v-else-if="account">
      <div class="card" style="margin-bottom:16px;">
        <div class="card-header"><h3 class="card-title">基本信息</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">显示名</span><br><strong>{{ account.display_name }}</strong></div>
            <div><span style="color:var(--text-secondary);">Username</span><br>{{ account.username ? '@' + account.username : '-' }}</div>
            <div><span style="color:var(--text-secondary);">用户 ID</span><br><code>{{ account.user_id }}</code></div>
            <div><span style="color:var(--text-secondary);">状态</span><br><span :class="['badge', account.status === 'active' ? 'badge-success' : 'badge-warning']">{{ account.status }}</span></div>
          </div>
        </div>
      </div>

      <div class="card" style="margin-bottom:16px;">
        <div class="card-header"><h3 class="card-title">Session 信息</h3></div>
        <div class="card-body">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div><span style="color:var(--text-secondary);">Session 状态</span><br><span :class="['badge', account.session_status === 'active' ? 'badge-success' : 'badge-warning']">{{ account.session_status || '无' }}</span></div>
            <div><span style="color:var(--text-secondary);">最后同步</span><br>{{ account.last_sync || '-' }}</div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header"><h3 class="card-title" style="color:var(--color-danger);">危险操作</h3></div>
        <div class="card-body">
          <p style="color:var(--text-secondary);margin-bottom:16px;">以下操作不可逆，请谨慎操作。</p>
          <div style="display:flex;gap:12px;">
            <button class="btn btn-outline" disabled title="功能开发中">远端 Logout</button>
            <button class="btn btn-danger" disabled title="功能开发中">删除本地 Session</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
