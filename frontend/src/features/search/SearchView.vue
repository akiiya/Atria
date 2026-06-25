<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from '@/i18n'
import { searchMessages, type SearchResult } from '@/api/search'
import EmptyState from '@/components/EmptyState.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

const { t } = useI18n()
const router = useRouter()

const query = ref('')
const results = ref<SearchResult[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const searched = ref(false)
const limit = 20
const offset = ref(0)

async function doSearch() {
  const q = query.value.trim()
  if (!q) {
    results.value = []
    total.value = 0
    searched.value = false
    return
  }

  loading.value = true
  error.value = ''
  try {
    const resp = await searchMessages(q, undefined, limit, offset.value)
    if (resp.ok) {
      results.value = resp.results
      total.value = resp.total
      searched.value = true
    } else {
      error.value = '搜索失败'
    }
  } catch {
    error.value = '搜索失败'
  }
  loading.value = false
}

function loadMore() {
  offset.value += limit
  doSearch()
}

function loadPrev() {
  offset.value = Math.max(0, offset.value - limit)
  doSearch()
}

function goToChat(peerRef: string) {
  router.push(`/chats/${peerRef}`)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    offset.value = 0
    doSearch()
  }
}
</script>

<template>
  <div class="search-page">
    <div class="search-header">
      <h1 class="search-title">{{ t('search.title') }}</h1>
    </div>

    <div class="search-input-wrap">
      <input
        v-model="query"
        class="search-input"
        type="text"
        :placeholder="t('search.placeholder')"
        @keydown="onKeydown"
      />
      <button class="btn btn-primary search-btn" @click="offset = 0; doSearch()" :disabled="loading || !query.trim()">
        {{ t('common.search') }}
      </button>
    </div>

    <!-- 错误 -->
    <div v-if="error" style="padding: 0 20px;">
      <ErrorBanner :message="error" @dismiss="error = ''" />
    </div>

    <!-- 未搜索 -->
    <div v-if="!searched && !loading" class="search-body">
      <EmptyState
        icon="&#x1f50d;"
        :title="t('search.title')"
        :description="t('search.placeholder')"
      />
    </div>

    <!-- 加载中 -->
    <div v-else-if="loading" class="search-body">
      <div class="loading-text">{{ t('common.loading') }}</div>
    </div>

    <!-- 无结果 -->
    <div v-else-if="results.length === 0" class="search-body">
      <EmptyState
        icon="&#x1f4ed;"
        :title="t('search.noResults')"
        :description="t('search.noResultsDesc').replace('{query}', query)"
      />
    </div>

    <!-- 结果列表 -->
    <div v-else class="search-results">
      <div class="search-total">{{ t('search.total').replace('{count}', String(total)) }}</div>
      <div
        v-for="(r, idx) in results"
        :key="idx"
        class="search-result-item"
        @click="goToChat(r.peer_ref)"
      >
        <div class="result-header">
          <span class="result-sender">{{ r.sender_name || r.peer_ref }}</span>
          <span class="result-time">{{ r.sent_at }}</span>
        </div>
        <div class="result-snippet" v-html="r.text_snippet"></div>
        <div class="result-meta">
          <span class="result-peer">{{ r.peer_ref }}</span>
          <span v-if="r.is_outgoing" class="result-outgoing">&#x2191; {{ t('search.sender') }}</span>
        </div>
      </div>

      <!-- 分页 -->
      <div class="search-pagination" v-if="total > limit">
        <button class="btn btn-sm btn-outline" @click="loadPrev" :disabled="offset === 0">
          {{ t('common.prev') }}
        </button>
        <span class="page-info">{{ offset + 1 }}-{{ Math.min(offset + limit, total) }} / {{ total }}</span>
        <button class="btn btn-sm btn-outline" @click="loadMore" :disabled="offset + limit >= total">
          {{ t('common.next') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.search-header {
  padding: 16px 20px 8px;
  flex-shrink: 0;
}

.search-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
}

.search-input-wrap {
  display: flex;
  gap: 8px;
  padding: 0 20px 12px;
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  padding: 10px 14px;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 14px;
  font-family: var(--font-sans);
  outline: none;
  transition: border-color 0.15s;
}

.search-input:focus {
  border-color: var(--accent-color);
}

.search-btn {
  flex-shrink: 0;
}

.search-body {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow-y: auto;
  padding: 20px;
}

.loading-text {
  color: var(--text-secondary);
  font-size: 14px;
}

.search-results {
  flex: 1;
  overflow-y: auto;
  padding: 0 20px 20px;
}

.search-total {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 12px;
}

.search-result-item {
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.15s;
}

.search-result-item:hover {
  background: var(--bg-secondary);
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.result-sender {
  font-weight: 600;
  font-size: 14px;
}

.result-time {
  font-size: 12px;
  color: var(--text-secondary);
}

.result-snippet {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.5;
  margin-bottom: 4px;
  word-break: break-word;
}

.result-meta {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--text-tertiary);
}

.result-outgoing {
  color: var(--accent-color);
}

.search-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 16px 0;
}

.page-info {
  font-size: 13px;
  color: var(--text-secondary);
}
</style>
