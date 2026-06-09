<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiGet } from '@/api/http'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'

const { data, isLoading, error } = useQuery({
  queryKey: ['audit-logs'],
  queryFn: () => apiGet<any>('/api/audit'),
  retry: 1,
})

const logs = computed(() => data.value?.logs || [])
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">审计日志</h1>
      <p class="page-desc">系统操作记录</p>
    </div>

    <div v-if="isLoading"><LoadingSkeleton /></div>
    <div v-else-if="error" class="alert alert-error">加载失败</div>

    <div v-else>
      <div v-if="logs.length === 0" class="card">
        <div class="card-body" style="text-align:center;padding:48px 24px;">
          <div style="font-size:48px;margin-bottom:16px;">📋</div>
          <div style="font-size:18px;font-weight:600;margin-bottom:8px;">暂无审计事件</div>
        </div>
      </div>

      <div v-else class="card">
        <div class="card-header"><h3 class="card-title">审计事件</h3></div>
        <div class="card-body" style="padding:0;">
          <table class="table">
            <thead>
              <tr>
                <th>时间</th>
                <th>操作</th>
                <th>资源</th>
                <th>风险等级</th>
                <th>IP</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="log in logs" :key="log.id">
                <td style="color:var(--text-secondary);white-space:nowrap;">{{ log.created_at }}</td>
                <td><strong>{{ log.action }}</strong></td>
                <td>{{ log.resource_type }} #{{ log.resource_id }}</td>
                <td>
                  <span :class="['badge', log.risk_level === 'high' || log.risk_level === 'critical' ? 'badge-danger' : log.risk_level === 'medium' ? 'badge-warning' : 'badge-success']">
                    {{ log.risk_level }}
                  </span>
                </td>
                <td style="color:var(--text-secondary);">{{ log.ip || '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
