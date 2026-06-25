<script setup lang="ts">
import { ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useI18n } from '@/i18n'
import {
  fetchMaintenanceStatus,
  cleanupChatCache,
  cleanupOrphans,
  cleanupMediaCache,
  type CleanupResult,
  type MediaCleanupResult,
} from '@/api/maintenance'

const { t } = useI18n()
const queryClient = useQueryClient()

const { data, isLoading, error, refetch } = useQuery({
  queryKey: ['maintenance-status'],
  queryFn: fetchMaintenanceStatus,
  refetchInterval: 30_000,
})

// Chat cache cleanup
const chatAccountId = ref<number | null>(null)
const chatPeerRef = ref('')
const chatCacheResult = ref<CleanupResult | null>(null)

const chatCacheMutation = useMutation({
  mutationFn: (dryRun: boolean) =>
    cleanupChatCache({
      account_id: chatAccountId.value ?? 0,
      peer_ref: chatPeerRef.value || undefined,
      dry_run: dryRun,
    }),
  onSuccess: (result) => {
    chatCacheResult.value = result
    if (!result.dry_run) {
      queryClient.invalidateQueries({ queryKey: ['maintenance-status'] })
    }
  },
})

function previewChatCache() {
  chatCacheMutation.mutate(true)
}

function executeChatCache() {
  if (!confirm(t('maintenance.confirmExecute'))) return
  chatCacheMutation.mutate(false)
}

// Orphan cleanup
const orphanResult = ref<CleanupResult | null>(null)

const orphanMutation = useMutation({
  mutationFn: (dryRun: boolean) => cleanupOrphans(dryRun),
  onSuccess: (result) => {
    orphanResult.value = result
    if (!result.dry_run) {
      queryClient.invalidateQueries({ queryKey: ['maintenance-status'] })
    }
  },
})

function previewOrphans() {
  orphanMutation.mutate(true)
}

function executeOrphans() {
  if (!confirm(t('maintenance.confirmExecute'))) return
  orphanMutation.mutate(false)
}

// Media cache cleanup
const mediaOnlyFailed = ref(false)
const mediaCacheResult = ref<MediaCleanupResult | null>(null)

const mediaCacheMutation = useMutation({
  mutationFn: (dryRun: boolean) =>
    cleanupMediaCache({
      only_failed: mediaOnlyFailed.value,
      dry_run: dryRun,
    }),
  onSuccess: (result) => {
    mediaCacheResult.value = result
    if (!result.dry_run) {
      queryClient.invalidateQueries({ queryKey: ['maintenance-status'] })
    }
  },
})

function previewMediaCache() {
  mediaCacheMutation.mutate(true)
}

function executeMediaCache() {
  if (!confirm(t('maintenance.confirmExecute'))) return
  mediaCacheMutation.mutate(false)
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t('maintenance.title') }}</h1>
      <p class="page-desc">{{ t('maintenance.desc') }}</p>
    </div>

    <div v-if="isLoading" class="stats-grid">
      <div v-for="i in 5" :key="i" class="card stat-card">
        <div class="skeleton-line long" style="height:32px;width:48px;margin:0 auto 8px"></div>
        <div class="skeleton-line short" style="height:14px;width:60px;margin:0 auto"></div>
      </div>
    </div>

    <div v-else-if="error" class="alert alert-error">{{ t('common.error') }}</div>

    <div v-else>
      <!-- Table Statistics -->
      <div class="stats-grid">
        <div class="card stat-card">
          <div class="stat-icon">📱</div>
          <div class="stat-value">{{ data?.account_count ?? 0 }}</div>
          <div class="stat-label">{{ t('maintenance.accounts') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">🔑</div>
          <div class="stat-value">{{ data?.api_key_count ?? 0 }}</div>
          <div class="stat-label">{{ t('maintenance.apiKeys') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">💬</div>
          <div class="stat-value">{{ data?.peer_cache_count ?? 0 }}</div>
          <div class="stat-label">{{ t('maintenance.peerCache') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">📨</div>
          <div class="stat-value">{{ data?.message_cache_count ?? 0 }}</div>
          <div class="stat-label">{{ t('maintenance.messageCache') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">📋</div>
          <div class="stat-value">{{ data?.audit_log_count ?? 0 }}</div>
          <div class="stat-label">{{ t('maintenance.auditLogs') }}</div>
        </div>
        <div class="card stat-card">
          <div class="stat-icon">🖼️</div>
          <div class="stat-value">{{ data?.media_cached_count ?? 0 }}</div>
          <div class="stat-label">{{ t('maintenance.mediaCache') }}</div>
        </div>
      </div>

      <!-- Cache Statistics -->
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:24px;">
        <!-- Orphan cache -->
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">{{ t('maintenance.cacheStats') }}</h3>
          </div>
          <div class="card-body">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px;">
              <div>
                <span style="color:var(--text-secondary);">{{ t('maintenance.orphanPeers') }}</span><br>
                <strong>{{ data?.orphan_peers ?? 0 }}</strong>
              </div>
              <div>
                <span style="color:var(--text-secondary);">{{ t('maintenance.orphanMessages') }}</span><br>
                <strong>{{ data?.orphan_messages ?? 0 }}</strong>
              </div>
              <div>
                <span style="color:var(--text-secondary);">{{ t('maintenance.migrationVersion') }}</span><br>
                <strong>v{{ data?.migration_version ?? 0 }}</strong>
              </div>
            </div>

            <div style="border-top:1px solid var(--border);padding-top:16px;">
              <h4 style="margin-bottom:8px;">{{ t('maintenance.cleanupOrphans') }}</h4>
              <p style="color:var(--text-secondary);font-size:13px;margin-bottom:12px;">
                {{ t('maintenance.cleanupOrphansDesc') }}
              </p>
              <div style="display:flex;gap:8px;">
                <button class="btn btn-sm btn-outline" :disabled="orphanMutation.isPending.value" @click="previewOrphans">
                  {{ t('maintenance.dryRun') }}
                </button>
                <button class="btn btn-sm btn-danger" :disabled="orphanMutation.isPending.value" @click="executeOrphans">
                  {{ t('maintenance.execute') }}
                </button>
              </div>
              <div v-if="orphanResult" class="alert alert-success" style="margin-top:12px;">
                {{ t('maintenance.previewResult') }}: {{ orphanResult.message }}
              </div>
            </div>
          </div>
        </div>

        <!-- Chat cache cleanup -->
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">{{ t('maintenance.cleanupChatCache') }}</h3>
          </div>
          <div class="card-body">
            <p style="color:var(--text-secondary);font-size:13px;margin-bottom:12px;">
              {{ t('maintenance.cleanupChatCacheDesc') }}
            </p>
            <div style="display:flex;flex-direction:column;gap:8px;margin-bottom:12px;">
              <div>
                <label style="font-size:13px;color:var(--text-secondary);">{{ t('maintenance.accountId') }} *</label>
                <input
                  v-model.number="chatAccountId"
                  type="number"
                  min="1"
                  class="form-input"
                  style="width:100%;margin-top:4px;"
                >
              </div>
              <div>
                <label style="font-size:13px;color:var(--text-secondary);">{{ t('maintenance.peerRef') }}</label>
                <input
                  v-model="chatPeerRef"
                  type="text"
                  class="form-input"
                  style="width:100%;margin-top:4px;"
                >
              </div>
            </div>
            <div style="display:flex;gap:8px;">
              <button
                class="btn btn-sm btn-outline"
                :disabled="!chatAccountId || chatCacheMutation.isPending.value"
                @click="previewChatCache"
              >
                {{ t('maintenance.dryRun') }}
              </button>
              <button
                class="btn btn-sm btn-danger"
                :disabled="!chatAccountId || chatCacheMutation.isPending.value"
                @click="executeChatCache"
              >
                {{ t('maintenance.execute') }}
              </button>
            </div>
            <div v-if="chatCacheResult" class="alert alert-success" style="margin-top:12px;">
              {{ t('maintenance.previewResult') }}: {{ chatCacheResult.message }}
            </div>
          </div>
        </div>

        <!-- Media cache cleanup -->
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">{{ t('maintenance.cleanupMediaCache') }}</h3>
          </div>
          <div class="card-body">
            <p style="color:var(--text-secondary);font-size:13px;margin-bottom:12px;">
              {{ t('maintenance.cleanupMediaCacheDesc') }}
            </p>
            <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;margin-bottom:12px;">
              <div>
                <span style="color:var(--text-secondary);">{{ t('maintenance.mediaRecords') }}</span><br>
                <strong>{{ data?.media_record_count ?? 0 }}</strong>
              </div>
              <div>
                <span style="color:var(--text-secondary);">{{ t('maintenance.mediaCached') }}</span><br>
                <strong>{{ data?.media_cached_count ?? 0 }}</strong>
              </div>
              <div>
                <span style="color:var(--text-secondary);">{{ t('maintenance.mediaFailed') }}</span><br>
                <strong>{{ data?.media_failed_count ?? 0 }}</strong>
              </div>
            </div>
            <div style="margin-bottom:12px;">
              <label style="font-size:13px;color:var(--text-secondary);display:flex;align-items:center;gap:6px;">
                <input v-model="mediaOnlyFailed" type="checkbox">
                {{ t('maintenance.onlyFailed') }}
              </label>
            </div>
            <div style="display:flex;gap:8px;">
              <button
                class="btn btn-sm btn-outline"
                :disabled="mediaCacheMutation.isPending.value"
                @click="previewMediaCache"
              >
                {{ t('maintenance.dryRun') }}
              </button>
              <button
                class="btn btn-sm btn-danger"
                :disabled="mediaCacheMutation.isPending.value"
                @click="executeMediaCache"
              >
                {{ t('maintenance.execute') }}
              </button>
            </div>
            <div v-if="mediaCacheResult" class="alert alert-success" style="margin-top:12px;">
              {{ t('maintenance.previewResult') }}: {{ mediaCacheResult.message }}
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Maintenance -->
      <div class="card" style="margin-top:16px;">
        <div class="card-header">
          <h3 class="card-title">{{ t('maintenance.recentMaintenance') }}</h3>
          <button class="btn btn-sm btn-outline" @click="refetch()">{{ t('common.refresh') }}</button>
        </div>
        <div class="card-body" style="padding:0;">
          <div v-if="!data?.recent_maintenance?.length" style="text-align:center;padding:24px;color:var(--text-secondary);">
            {{ t('maintenance.noMaintenance') }}
          </div>
          <table v-else class="table">
            <tbody>
              <tr v-for="item in data.recent_maintenance" :key="item.id">
                <td style="white-space:nowrap;color:var(--text-secondary);font-size:12px;">{{ item.created_at }}</td>
                <td style="font-size:13px;">{{ t('event.' + item.action) || item.action }}</td>
                <td style="font-size:13px;">{{ item.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
