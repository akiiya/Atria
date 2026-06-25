<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { useAccountStore } from '@/stores/account'
import { fetchContacts } from '@/api/contacts'
import { useI18n } from '@/i18n'
import ErrorBanner from '@/components/ErrorBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import type { Contact } from '@/types/contacts'

const { t } = useI18n()
const router = useRouter()
const account = useAccountStore()
const searchQuery = ref('')

const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['contacts', account.currentAccountId]),
  queryFn: () => fetchContacts(),
  enabled: computed(() => !!account.currentAccountId),
  retry: 1,
  staleTime: 60_000,
  refetchOnWindowFocus: false,
})

const contacts = computed(() => data.value?.contacts || [])

const filteredContacts = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return contacts.value
  return contacts.value.filter(c =>
    c.display_name.toLowerCase().includes(q) ||
    (c.username && c.username.toLowerCase().includes(q)) ||
    (c.phone && c.phone.includes(q))
  )
})

function goToChat(contact: Contact) {
  router.push(`/chats/${contact.peer_ref}`)
}

function getInitial(name: string): string {
  if (!name) return '?'
  const chars = Array.from(name)
  return chars[0] || '?'
}
</script>

<template>
  <div class="contacts-page">
    <div class="contacts-header">
      <h1 class="contacts-title">{{ t('contacts.title') }}</h1>
      <span class="contacts-count" v-if="contacts.length">{{ t('contacts.count').replace('{count}', String(contacts.length)) }}</span>
    </div>

    <!-- 搜索栏 -->
    <div class="contacts-search">
      <input
        v-model="searchQuery"
        class="contacts-search-input"
        type="text"
        :placeholder="t('contacts.search')"
      />
    </div>

    <!-- 无账号 -->
    <div v-if="!account.currentAccountId" class="contacts-body">
      <EmptyState
        icon="🔑"
        :title="t('contacts.noAccount')"
        :description="t('contacts.noAccountDesc')"
      />
    </div>

    <!-- 加载中 -->
    <div v-else-if="isLoading" class="contacts-body">
      <div class="skeleton-list">
        <div v-for="i in 12" :key="i" class="skeleton-item">
          <div class="skeleton-avatar"></div>
          <div class="skeleton-lines">
            <div class="skeleton-line long"></div>
            <div class="skeleton-line short"></div>
          </div>
        </div>
      </div>
    </div>

    <!-- 错误 -->
    <div v-else-if="error" class="contacts-body">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>

    <!-- 空列表 -->
    <div v-else-if="contacts.length === 0" class="contacts-body">
      <EmptyState
        icon="👥"
        :title="t('contacts.empty')"
        :description="t('contacts.emptyDesc')"
      />
    </div>

    <!-- 搜索无结果 -->
    <div v-else-if="filteredContacts.length === 0" class="contacts-body">
      <EmptyState
        icon="🔍"
        :title="t('contacts.noResults')"
        :description="t('contacts.noResultsDesc').replace('{query}', searchQuery)"
      />
    </div>

    <!-- 联系人列表 -->
    <div v-else class="contacts-list">
      <div
        v-for="contact in filteredContacts"
        :key="contact.peer_ref"
        class="contact-item"
        @click="goToChat(contact)"
      >
        <div class="contact-avatar">
          {{ contact.avatar_initial || getInitial(contact.display_name) }}
        </div>
        <div class="contact-info">
          <div class="contact-name-row">
            <span class="contact-name">{{ contact.display_name }}</span>
            <span v-if="contact.has_dialog" class="contact-badge">{{ t('contacts.hasDialog') }}</span>
          </div>
          <div class="contact-meta">
            <span v-if="contact.username" class="contact-username">@{{ contact.username }}</span>
            <span v-if="contact.phone" class="contact-phone">{{ contact.phone }}</span>
          </div>
        </div>
        <div class="contact-action">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M9 18l6-6-6-6"/>
          </svg>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.contacts-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.contacts-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 20px 8px;
  flex-shrink: 0;
}

.contacts-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
}

.contacts-count {
  font-size: 13px;
  color: var(--text-secondary);
}

.contacts-search {
  padding: 0 20px 12px;
  flex-shrink: 0;
}

.contacts-search-input {
  width: 100%;
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

.contacts-search-input:focus {
  border-color: var(--accent-color);
}

.contacts-body {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow-y: auto;
  padding: 20px;
}

.contacts-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px;
}

.contact-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.15s;
}

.contact-item:hover {
  background: var(--bg-secondary);
}

.contact-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--accent-color);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 600;
  flex-shrink: 0;
}

.contact-info {
  flex: 1;
  min-width: 0;
}

.contact-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.contact-name {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.contact-badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 6px;
  background: var(--color-success-light, rgba(16, 185, 129, 0.1));
  color: var(--color-success, #10b981);
  flex-shrink: 0;
}

.contact-meta {
  display: flex;
  gap: 8px;
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-secondary);
}

.contact-username {
  color: var(--accent-color);
}

.contact-action {
  color: var(--text-tertiary);
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}

.contact-item:hover .contact-action {
  opacity: 1;
}
</style>
