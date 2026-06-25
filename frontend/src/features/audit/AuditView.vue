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

const filters = computed(() => ({
  action: filterAction.value,
  account_id: filterAccountId.value,
  risk_level: filterRiskLevel.value,
  offset: offset.value,
  limit,
}))

const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['audit-logs', filters.value]),
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

// 预定义的事件类型（用于筛选下拉）
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
  offset.value = 0
}

function nextPage() {
  if (hasMore.value) {
    offset.value += limit
  }
}

function prevPage() {
  offset.value = Math.max(0, offset.value - limit)
}

// 筛选变化时重置分页
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
  <div class="audit-page">
    <div class="audit-header">
      <h1 class="audit-title">审计日志</h1>
      <span class="audit-count" v-if="total > 0">共 {{ total }} 条</span>
    </div>

    <!-- 筛选栏 -->
    <div class="audit-filters">
      <select v-model="filterAction" class="filter-select">
        <option v-for="t in eventTypes" :key="t.value" :value="t.value">{{ t.label }}</option>
      </select>
      <select v-model="filterRiskLevel" class="filter-select">
        <option v-for="r in riskLevels" :key="r.value" :value="r.value">{{ r.label }}</option>
      </select>
      <input
        v-model="filterAccountId"
        class="filter-input"
        type="text"
        placeholder="账号 ID"
      />
      <button class="btn-text" @click="resetFilters">重置</button>
    </div>

    <!-- 加载中 -->
    <div v-if="isLoading"><LoadingSkeleton /></div>

    <!-- 错误 -->
    <div v-else-if="error" class="audit-body">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>

    <!-- 空列表 -->
    <div v-else-if="logs.length === 0" class="audit-body">
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
              <td class="col-time">{{ formatTime(log.created_at) }}</td>
              <td><span class="action-tag">{{ actionLabel(log.action) }}</span></td>
              <td class="col-resource">{{ log.resource_type }}<span v-if="log.resource_id"> #{{ log.resource_id }}</span></td>
              <td class="col-account">{{ log.account_id || '-' }}</td>
              <td>
                <span :class="['badge', riskBadgeClass(log.risk_level)]">
                  {{ log.risk_level }}
                </span>
              </td>
              <td class="col-ip">{{ log.ip || '-' }}</td>
              <td class="col-message" :title="log.message">{{ log.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页 -->
      <div class="audit-pagination">
        <button class="btn-text" :disabled="offset === 0" @click="prevPage">← 上一页</button>
        <span class="pagination-info">
          {{ offset + 1 }}-{{ Math.min(offset + limit, total) }} / {{ total }}
        </span>
        <button class="btn-text" :disabled="!hasMore" @click="nextPage">下一页 →</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.audit-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 16px;
}

.audit-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 0 8px;
}

.audit-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
}

.audit-count {
  font-size: 13px;
  color: var(--text-secondary);
}

.audit-filters {
  display: flex;
  gap: 8px;
  padding: 8px 0 16px;
  flex-wrap: wrap;
  align-items: center;
}

.filter-select,
.filter-input {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 13px;
  font-family: var(--font-sans);
  outline: none;
  min-width: 120px;
}

.filter-select:focus,
.filter-input:focus {
  border-color: var(--accent-color);
}

.filter-input {
  width: 100px;
  min-width: 80px;
}

.btn-text {
  background: none;
  border: none;
  color: var(--accent-color);
  cursor: pointer;
  font-size: 13px;
  padding: 6px 10px;
  border-radius: 6px;
  transition: background 0.15s;
}

.btn-text:hover {
  background: var(--bg-secondary);
}

.btn-text:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.audit-body {
  padding: 20px 0;
}

.col-time {
  color: var(--text-secondary);
  white-space: nowrap;
  font-size: 12px;
}

.col-resource {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
}

.col-account {
  font-size: 12px;
  color: var(--text-secondary);
}

.col-ip {
  color: var(--text-secondary);
  font-size: 12px;
  font-family: var(--font-mono, monospace);
}

.col-message {
  max-width: 250px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--text-secondary);
}

.action-tag {
  font-size: 12px;
  font-weight: 500;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-secondary);
}

.audit-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 12px 16px;
  border-top: 1px solid var(--border-color);
}

.pagination-info {
  font-size: 13px;
  color: var(--text-secondary);
}
</style>
