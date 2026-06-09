<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { fetchDashboardStats } from '@/api/me'

const { data, isLoading, error } = useQuery({
  queryKey: ['dashboard-stats'],
  queryFn: fetchDashboardStats,
})
</script>

<template>
  <div>
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
        <div class="stat-value">{{ data?.api_key_count ?? 0 }}</div>
        <div class="stat-label">API 凭据</div>
      </div>
      <div class="card stat-card">
        <div class="stat-icon">📱</div>
        <div class="stat-value">{{ data?.account_count ?? 0 }}</div>
        <div class="stat-label">已登录账号</div>
      </div>
      <div class="card stat-card">
        <div class="stat-icon">🔗</div>
        <div class="stat-value">{{ data?.session_count ?? 0 }}</div>
        <div class="stat-label">活跃 Session</div>
      </div>
      <div class="card stat-card">
        <div class="stat-icon">📋</div>
        <div class="stat-value">{{ data?.audit_today ?? 0 }}</div>
        <div class="stat-label">今日审计事件</div>
      </div>
    </div>

    <div style="display:grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 24px;">
      <div class="card">
        <div class="card-header"><h3 class="card-title">安全提示</h3></div>
        <div class="card-body">
          <div style="display:flex;flex-direction:column;gap:16px;">
            <div style="display:flex;gap:12px;align-items:flex-start;">
              <span style="font-size:20px;">🔒</span>
              <div><strong>备份密钥</strong><p style="color:var(--text-secondary);margin:4px 0 0;">请确保已备份 <code>data/secret.key</code> 文件。丢失密钥将无法恢复加密的 Session 数据。</p></div>
            </div>
            <div style="display:flex;gap:12px;align-items:flex-start;">
              <span style="font-size:20px;">🛡️</span>
              <div><strong>风险策略</strong><p style="color:var(--text-secondary);margin:4px 0 0;">所有 API 凭据默认禁止高风险操作。如需启用，请在凭据设置中配置风险策略。</p></div>
            </div>
            <div style="display:flex;gap:12px;align-items:flex-start;">
              <span style="font-size:20px;">📝</span>
              <div><strong>审计日志</strong><p style="color:var(--text-secondary);margin:4px 0 0;">所有管理操作都会记录在审计日志中，便于追踪和排查问题。</p></div>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header"><h3 class="card-title">快速开始</h3></div>
        <div class="card-body">
          <div style="display:flex;flex-direction:column;gap:16px;">
            <div style="display:flex;gap:12px;align-items:flex-start;">
              <div style="width:28px;height:28px;border-radius:50%;background:var(--accent-color);color:#fff;display:flex;align-items:center;justify-content:center;font-weight:600;font-size:13px;flex-shrink:0;">1</div>
              <div><strong>配置 API 凭据</strong><p style="color:var(--text-secondary);margin:4px 0 0;">添加 Telegram API ID 和 API Hash</p></div>
            </div>
            <div style="display:flex;gap:12px;align-items:flex-start;">
              <div style="width:28px;height:28px;border-radius:50%;background:var(--accent-color);color:#fff;display:flex;align-items:center;justify-content:center;font-weight:600;font-size:13px;flex-shrink:0;">2</div>
              <div><strong>登录账号</strong><p style="color:var(--text-secondary);margin:4px 0 0;">使用手机号登录 Telegram 账号</p></div>
            </div>
            <div style="display:flex;gap:12px;align-items:flex-start;">
              <div style="width:28px;height:28px;border-radius:50%;background:var(--accent-color);color:#fff;display:flex;align-items:center;justify-content:center;font-weight:600;font-size:13px;flex-shrink:0;">3</div>
              <div><strong>管理会话</strong><p style="color:var(--text-secondary);margin:4px 0 0;">查看和管理已登录的账号状态</p></div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:16px;">
      <div class="card-header"><h3 class="card-title">系统信息</h3></div>
      <div class="card-body">
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
          <div><span style="color:var(--text-secondary);">版本</span><br><strong>{{ data?.version || 'v0.1.0-dev' }}</strong></div>
          <div><span style="color:var(--text-secondary);">数据库</span><br><strong>{{ data?.db_driver || 'sqlite' }}</strong></div>
          <div><span style="color:var(--text-secondary);">数据目录</span><br><code>{{ data?.data_dir || './data' }}</code></div>
          <div><span style="color:var(--text-secondary);">监听地址</span><br><code>{{ data?.listen_addr || '127.0.0.1:8080' }}</code></div>
        </div>
      </div>
    </div>
  </div>
</template>
