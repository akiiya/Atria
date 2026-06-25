<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiGet } from '@/api/http'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

interface AuditLog {
  id: number
  account_id: number
  action: string
  resource_type: string
  resource_id: number
  risk_level: string
  ip: string
  message: string
  metadata_json: string
  created_at: string
}

interface AuditResponse {
  ok: boolean
  logs: AuditLog[]
  total: number
  limit: number
  offset: number
}

// 筛选条件
const filterAction = ref('')
const filterAccountId = ref('')
const filterRiskLevel = ref('')
const offset = ref(0)
const limit = 50

const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['audit-logs', filterAction.value, filterAccountId.value, filterRiskLevel.value, offset.value]),
  queryFn: () => {
    let url = `/api/audit?limit=${limit}&offset=${offset.value}`
    if (filterAction.value) url += `&action=${encodeURIComponent(filterAction.value)}`
    if (filterAccountId.value) url += `&account_id=${encodeURIComponent(filterAccountId.value)}`
    if (filterRiskLevel.value) url += `&risk_level=${encodeURIComponent(filterRiskLevel.value)}`
    return apiGet<AuditResponse>(url)
  },
  retry: 1,
})

const logs = computed(() => data.value?.logs || [])
const total = computed(() => data.value?.total || 0)
const hasMore = computed(() => offset.value + limit < total.value)

const eventTypes = [
  { value: '', label: '全部类型' },
  { value: 'admin.login', label: '管理员登录' },
  { value: 'admin.logout', label: '管理员登出' },
  { value: 'admin.init', label: '初始化管理员' },
  { value: 'api_credential.create', label: '创建 API Key' },
  { value: 'api_credential.update', label: '更新 API Key' },
  { value: 'api_credential.delete', label: '删除 API Key' },
  { value: 'account.login_start', label: '开始登录账号' },
  { value: 'account.login_authorized', label: '账号授权成功' },
  { value: 'account.select', label: '切换当前账号' },
  { value: 'runtime.start', label: '启动 Runtime' },
  { value: 'runtime.stop', label: '停止 Runtime' },
  { value: 'settings.proxy.save', label: '保存代理配置' },
  { value: 'chat.send_message', label: '发送消息' },
]

const riskLevels = [
  { value: '', label: '全部等级' },
  { value: 'low', label: '低' },
  { value: 'medium', label: '中' },
  { value: 'high', label: '高' },
  { value: 'critical', label: '严重' },
]

function resetFilters() {
  filterAction.value = ''
  filterAccountId.value = ''
  filterRiskLevel.value = ''
}

function nextPage() {
  if (hasMore.value) offset.value += limit
}

function prevPage() {
  offset.value = Math.max(0, offset.value - limit)
}

watch([filterAction, filterAccountId, filterRiskLevel], () => {
  offset.value = 0
})

function formatTime(at: string): string {
  if (!at) return '-'
  try {
    const d = new Date(at.replace(' ', 'T') + 'Z')
    return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return at
  }
}

function riskBadgeClass(level: string): string {
  if (level === 'high' || level === 'critical') return 'badge-danger'
  if (level === 'medium') return 'badge-warning'
  return 'badge-success'
}

function actionLabel(action: string): string {
  const found = eventTypes.find(e => e.value === action)
  return found ? found.label : action
}
</script>

<template>
  <div>
    <div class="page-header" style="display:flex;justify-content:space-between;align-items:flex-start;">
      <div>
        <h1 class="page-title">审计日志</h1>
        <p class="page-desc">系统操作记录与安全追踪</p>
      </div>
      <span v-if="total > 0" style="font-size:13px;color:var(--text-secondary);">共 {{ total }} 条</span>
    </div>

    <!-- 筛选栏 -->
    <div class="card" style="margin-bottom:16px;">
      <div class="card-body" style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;padding:12px 16px;">
        <select v-model="filterAction" class="form-input" style="width:auto;min-width:140px;padding:6px 10px;font-size:13px;">
          <option v-for="t in eventTypes" :key="t.value" :value="t.value">{{ t.label }}</option>
        </select>
        <select v-model="filterRiskLevel" class="form-input" style="width:auto;min-width:100px;padding:6px 10px;font-size:13px;">
          <option v-for="r in riskLevels" :key="r.value" :value="r.value">{{ r.label }}</option>
        </select>
        <input
          v-model="filterAccountId"
          class="form-input"
          type="text"
          placeholder="账号 ID"
          style="width:100px;min-width:80px;padding:6px 10px;font-size:13px;"
        />
        <button class="btn btn-sm btn-outline" @click="resetFilters">重置</button>
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="isLoading" class="card"><div class="card-body"><LoadingSkeleton /></div></div>

    <!-- 错误 -->
    <div v-else-if="error">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>

    <!-- 空列表 -->
    <div v-else-if="logs.length === 0" class="card">
      <EmptyState
        icon="📋"
        title="暂无审计事件"
        description="系统操作记录将在此显示。"
      />
    </div>

    <!-- 日志表格 -->
    <div v-else class="card">
      <div class="card-body" style="padding:0;">
        <table class="table">
          <thead>
            <tr>
              <th>时间</th>
              <th>操作</th>
              <th>资源</th>
              <th>账号</th>
              <th>等级</th>
              <th>IP</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in logs" :key="log.id">
              <td style="color:var(--text-secondary);white-space:nowrap;font-size:12px;">{{ formatTime(log.created_at) }}</td>
              <td><span style="font-size:12px;font-weight:500;padding:1px 6px;border-radius:4px;background:var(--bg-tertiary);">{{ actionLabel(log.action) }}</span></td>
              <td style="font-family:var(--font-mono,monospace);font-size:12px;">{{ log.resource_type }}<span v-if="log.resource_id"> #{{ log.resource_id }}</span></td>
              <td style="font-size:12px;color:var(--text-secondary);">{{ log.account_id || '-' }}</td>
              <td>
                <span :class="['badge', riskBadgeClass(log.risk_level)]">{{ log.risk_level }}</span>
              </td>
              <td style="color:var(--text-secondary);font-size:12px;font-family:var(--font-mono,monospace);">{{ log.ip || '-' }}</td>
              <td style="max-width:250px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:13px;color:var(--text-secondary);" :title="log.message">{{ log.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页 -->
      <div style="display:flex;align-items:center;justify-content:center;gap:16px;padding:12px 16px;border-top:1px solid var(--border-color);">
        <button class="btn btn-sm btn-outline" :disabled="offset === 0" @click="prevPage">← 上一页</button>
        <span style="font-size:13px;color:var(--text-secondary);">{{ offset + 1 }}-{{ Math.min(offset + limit, total) }} / {{ total }}</span>
        <button class="btn btn-sm btn-outline" :disabled="!hasMore" @click="nextPage">下一页 →</button>
      </div>
    </div>
  </div>
</template>
