<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiGet } from '@/api/http'
import { useI18n } from '@/i18n'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

const { t } = useI18n()

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

interface EventTypeOption {
  value: string
  label: string
}

// Fetch event types from API
const { data: eventTypesData } = useQuery({
  queryKey: ['audit-event-types'],
  queryFn: () => apiGet<{ ok: boolean; event_types: EventTypeOption[] }>('/api/audit/event-types'),
  retry: 1,
  staleTime: 60_000,
})

const eventTypes = computed(() => {
  const fromApi = eventTypesData.value?.event_types || []
  return [{ value: '', label: t('audit.allTypes') }, ...fromApi]
})

const riskLevels = computed(() => [
  { value: '', label: t('audit.allLevels') },
  { value: 'low', label: t('risk.low') },
  { value: 'medium', label: t('risk.medium') },
  { value: 'high', label: t('risk.high') },
  { value: 'critical', label: t('risk.critical') },
])

// Filters
const filterAction = ref('')
const filterAccountId = ref('')
const filterRiskLevel = ref('')
const filterSince = ref('')
const filterUntil = ref('')
const offset = ref(0)
const limit = 50

const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['audit-logs', filterAction.value, filterAccountId.value, filterRiskLevel.value, filterSince.value, filterUntil.value, offset.value]),
  queryFn: () => {
    let url = `/api/audit?limit=${limit}&offset=${offset.value}`
    if (filterAction.value) url += `&event_type=${encodeURIComponent(filterAction.value)}`
    if (filterAccountId.value) url += `&account_id=${encodeURIComponent(filterAccountId.value)}`
    if (filterRiskLevel.value) url += `&risk_level=${encodeURIComponent(filterRiskLevel.value)}`
    if (filterSince.value) url += `&since=${encodeURIComponent(filterSince.value)}`
    if (filterUntil.value) url += `&until=${encodeURIComponent(filterUntil.value)}`
    return apiGet<AuditResponse>(url)
  },
  retry: 1,
})

const logs = computed(() => data.value?.logs || [])
const total = computed(() => data.value?.total || 0)
const hasMore = computed(() => offset.value + limit < total.value)

function resetFilters() {
  filterAction.value = ''
  filterAccountId.value = ''
  filterRiskLevel.value = ''
  filterSince.value = ''
  filterUntil.value = ''
}

function nextPage() {
  if (hasMore.value) offset.value += limit
}

function prevPage() {
  offset.value = Math.max(0, offset.value - limit)
}

watch([filterAction, filterAccountId, filterRiskLevel, filterSince, filterUntil], () => {
  offset.value = 0
})

function formatTime(at: string): string {
  if (!at) return '-'
  try {
    const d = new Date(at.replace(' ', 'T') + 'Z')
    return d.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
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
  return t('event.' + action) || action
}
</script>

<template>
  <div>
    <div class="page-header" style="display:flex;justify-content:space-between;align-items:flex-start;">
      <div>
        <h1 class="page-title">{{ t('audit.title') }}</h1>
        <p class="page-desc">{{ t('audit.desc') }}</p>
      </div>
      <span v-if="total > 0" style="font-size:13px;color:var(--text-secondary);">{{ t('audit.total').replace('{count}', String(total)) }}</span>
    </div>

    <!-- Filter bar -->
    <div class="card" style="margin-bottom:16px;">
      <div class="card-body" style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;padding:12px 16px;">
        <select v-model="filterAction" class="form-input" style="width:auto;min-width:140px;padding:6px 10px;font-size:13px;">
          <option v-for="et in eventTypes" :key="et.value" :value="et.value">{{ et.label }}</option>
        </select>
        <select v-model="filterRiskLevel" class="form-input" style="width:auto;min-width:100px;padding:6px 10px;font-size:13px;">
          <option v-for="r in riskLevels" :key="r.value" :value="r.value">{{ r.label }}</option>
        </select>
        <input
          v-model="filterAccountId"
          class="form-input"
          type="text"
          :placeholder="t('audit.accountId')"
          style="width:100px;min-width:80px;padding:6px 10px;font-size:13px;"
        />
        <input v-model="filterSince" type="datetime-local" class="form-input" style="width:auto;padding:6px 10px;font-size:13px;" />
        <input v-model="filterUntil" type="datetime-local" class="form-input" style="width:auto;padding:6px 10px;font-size:13px;" />
        <button class="btn btn-sm btn-outline" @click="resetFilters">{{ t('common.reset') }}</button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="isLoading" class="card"><div class="card-body"><LoadingSkeleton /></div></div>

    <!-- Error -->
    <div v-else-if="error">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>

    <!-- Empty -->
    <div v-else-if="logs.length === 0" class="card">
      <EmptyState
        icon="📋"
        :title="t('audit.empty')"
        :description="t('audit.emptyDesc')"
      />
    </div>

    <!-- Log table -->
    <div v-else class="card">
      <div class="card-body" style="padding:0;">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('audit.time') }}</th>
              <th>{{ t('audit.action') }}</th>
              <th>{{ t('audit.resource') }}</th>
              <th>{{ t('audit.account') }}</th>
              <th>{{ t('audit.level') }}</th>
              <th>IP</th>
              <th>{{ t('audit.message') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in logs" :key="log.id">
              <td style="color:var(--text-secondary);white-space:nowrap;font-size:12px;">{{ formatTime(log.created_at) }}</td>
              <td><span style="font-size:12px;font-weight:500;padding:1px 6px;border-radius:4px;background:var(--bg-tertiary);">{{ actionLabel(log.action) }}</span></td>
              <td style="font-family:var(--font-mono,monospace);font-size:12px;">{{ log.resource_type }}<span v-if="log.resource_id"> #{{ log.resource_id }}</span></td>
              <td style="font-size:12px;color:var(--text-secondary);">{{ log.account_id || '-' }}</td>
              <td>
                <span :class="['badge', riskBadgeClass(log.risk_level)]">{{ t('risk.' + log.risk_level) || log.risk_level }}</span>
              </td>
              <td style="color:var(--text-secondary);font-size:12px;font-family:var(--font-mono,monospace);">{{ log.ip || '-' }}</td>
              <td style="max-width:250px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:13px;color:var(--text-secondary);" :title="log.message">{{ log.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div style="display:flex;align-items:center;justify-content:center;gap:16px;padding:12px 16px;border-top:1px solid var(--border-color);">
        <button class="btn btn-sm btn-outline" :disabled="offset === 0" @click="prevPage">&larr; {{ t('common.prev') }}</button>
        <span style="font-size:13px;color:var(--text-secondary);">{{ offset + 1 }}-{{ Math.min(offset + limit, total) }} / {{ total }}</span>
        <button class="btn btn-sm btn-outline" :disabled="!hasMore" @click="nextPage">{{ t('common.next') }} &rarr;</button>
      </div>
    </div>
  </div>
</template>
