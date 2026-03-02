import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      redirect: '/dashboard',
      children: [
        {
          path: 'dashboard',
          name: 'Dashboard',
          component: () => import('@/views/dashboard/DashboardView.vue'),
        },
        {
          path: 'tasks',
          name: 'Tasks',
          component: () => import('@/views/scan/TaskListView.vue'),
        },
        {
          path: 'tasks/:id',
          name: 'TaskDetail',
          component: () => import('@/views/findings/FindingsView.vue'),
        },
        {
          path: 'new-scan',
          name: 'NewScan',
          component: () => import('@/views/scan/NewScanView.vue'),
        },
      ],
    },
  ],
})

export default router
