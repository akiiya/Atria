import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory('/app/'),
  routes: [
    {
      path: '/',
      redirect: '/chats',
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/features/dashboard/DashboardView.vue'),
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
      path: '/accounts',
      name: 'accounts',
      component: () => import('@/features/dashboard/DashboardView.vue'),
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/features/dashboard/DashboardView.vue'),
    },
  ],
})

export default router
