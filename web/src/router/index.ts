import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/auth/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      redirect: '/dashboard',
      children: [
        { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/dashboard/DashboardView.vue') },
        { path: 'tasks',     name: 'Tasks',     component: () => import('@/views/scan/TaskListView.vue') },
        { path: 'tasks/:id', name: 'TaskDetail',component: () => import('@/views/findings/FindingsView.vue') },
        { path: 'new-scan',  name: 'NewScan',   component: () => import('@/views/scan/NewScanView.vue') },
        { path: 'rules',     name: 'Rules',     component: () => import('@/views/rules/RulesView.vue') },
        { path: 'vulnmap',   name: 'VulnMap',   component: () => import('@/views/vulnmap/VulnMapView.vue') },
        { path: 'settings',  name: 'Settings',  component: () => import('@/views/settings/SettingsView.vue') },
      ],
    },
  ],
})

// 路由守卫：未登录跳转到 /login
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.public) {
    // 已登录访问登录页，直接进主界面
    if (token) return next('/')
    return next()
  }
  if (!token) return next('/login')
  next()
})

export default router
