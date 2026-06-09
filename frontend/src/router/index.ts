import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory('/app/'),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/features/dashboard/DashboardView.vue'),
    },
    {
      path: '/accounts',
      name: 'accounts',
      component: () => import('@/features/accounts/AccountsView.vue'),
    },
    {
      path: '/accounts/login',
      name: 'account-login',
      component: () => import('@/features/accounts/AccountLoginView.vue'),
    },
    {
      path: '/accounts/:id',
      name: 'account-detail',
      component: () => import('@/features/accounts/AccountDetailView.vue'),
    },
    {
      path: '/chats',
      name: 'chats',
      component: () => import('@/features/chat/ChatView.vue'),
    },
    {
      path: '/chats/:peerRef',
      name: 'chat-detail',
      component: () => import('@/features/chat/ChatView.vue'),
    },
    {
      path: '/contacts',
      name: 'contacts',
      component: () => import('@/features/contacts/ContactsView.vue'),
    },
    {
      path: '/audit',
      name: 'audit',
      component: () => import('@/features/audit/AuditView.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/features/settings/SettingsView.vue'),
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/features/dashboard/DashboardView.vue'),
    },
  ],
})

export default router
