<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { fetchDashboardStats } from '@/api/me'

const { data, isLoading, error } = useQuery({
  queryKey: ['dashboard-stats'],
  queryFn: fetchDashboardStats,
})
</script>

<template>
  <div class="dashboard">
    <div class="page-header">
      <h1 class="page-title">仪表盘</h1>
      <p class="page-desc">系统概览与快速操作</p>
    </div>

    <div v-if="isLoading" class="stats-grid">
      <div v-for="i in 4" :key="i" class="card stat-card">
        <div class="skeleton-line long" style="height:32px;width:48px;margin:0 auto 8px"></div>
        <div class="skeleton-line short" style="height:14px;width:60px;margin:0 auto"></div>
      </div>
    </div>

    <div v-else-if="error" class="alert alert-error">加载失败</div>

    <div v-else class="stats-grid">
      <div class="card stat-card">
        <div class="stat-icon">🔑</div>
        <div class="stat-content">
          <div class="stat-value">{{ data?.api_key_count ?? 0 }}</div>
          <div class="stat-label">API 凭据</div>
        </div>
      </div>
      <div class="card stat-card">
        <div class="stat-icon">📱</div>
        <div class="stat-content">
          <div class="stat-value">{{ data?.account_count ?? 0 }}</div>
          <div class="stat-label">已登录账号</div>
        </div>
      </div>
      <div class="card stat-card">
        <div class="stat-icon">🔗</div>
        <div class="stat-content">
          <div class="stat-value">{{ data?.session_count ?? 0 }}</div>
          <div class="stat-label">活跃 Session</div>
        </div>
      </div>
      <div class="card stat-card">
        <div class="stat-icon">📋</div>
        <div class="stat-content">
          <div class="stat-value">{{ data?.audit_today ?? 0 }}</div>
          <div class="stat-label">今日审计事件</div>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:24px">
      <div class="card-header"><h3 class="card-title">快速开始</h3></div>
      <div class="card-body">
        <div class="quickstart-steps">
          <div class="step-item">
            <div class="step-number">1</div>
            <div class="step-content"><strong>配置 API 凭据</strong><p>添加 Telegram API ID 和 API Hash</p></div>
          </div>
          <div class="step-item">
            <div class="step-number">2</div>
            <div class="step-content"><strong>登录账号</strong><p>使用手机号登录 Telegram 账号</p></div>
          </div>
          <div class="step-item">
            <div class="step-number">3</div>
            <div class="step-content"><strong>开始聊天</strong><p>查看会话列表，发送消息</p></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dashboard { padding: 24px; max-width: 960px; }
.page-header { margin-bottom: 24px; }
.page-title { font-size: 24px; font-weight: 700; margin-bottom: 4px; }
.page-desc { color: var(--color-text-secondary); font-size: 14px; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; }
.stat-card { padding: 20px; text-align: center; }
.stat-icon { font-size: 32px; margin-bottom: 8px; }
.stat-value { font-size: 28px; font-weight: 700; }
.stat-label { font-size: 13px; color: var(--color-text-secondary); margin-top: 4px; }
.card { background: var(--color-surface); border: 1px solid var(--color-border); border-radius: 8px; }
.card-header { padding: 16px; border-bottom: 1px solid var(--color-border); }
.card-title { font-size: 15px; font-weight: 600; }
.card-body { padding: 16px; }
.quickstart-steps { display: flex; flex-direction: column; gap: 16px; }
.step-item { display: flex; align-items: flex-start; gap: 12px; }
.step-number {
  width: 28px; height: 28px; border-radius: 50%; background: var(--color-primary);
  color: #fff; display: flex; align-items: center; justify-content: center;
  font-weight: 600; font-size: 13px; flex-shrink: 0;
}
.step-content strong { display: block; margin-bottom: 2px; }
.step-content p { color: var(--color-text-secondary); font-size: 13px; margin: 0; }
</style>
