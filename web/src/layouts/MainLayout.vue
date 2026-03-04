<template>
  <div class="layout" :class="{ 'sidebar-collapsed': appStore.sidebarCollapsed }">
    <!-- 扫描线装饰 -->
    <div class="scan-line" />

    <!-- 侧边栏 -->
    <aside class="sidebar">
      <div class="sidebar-logo">
        <div class="logo-icon">
          <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
            <path d="M4 14 L14 4 L24 14 L14 24 Z" stroke="var(--accent-primary)" stroke-width="1.5" fill="none"/>
            <path d="M9 14 L14 9 L19 14 L14 19 Z" fill="var(--accent-primary)" opacity="0.6"/>
            <circle cx="14" cy="14" r="2" fill="var(--accent-primary)"/>
          </svg>
        </div>
        <transition name="logo-text">
          <div v-if="!appStore.sidebarCollapsed" class="logo-text">
            <span class="logo-primary">CodeQL</span>
            <span class="logo-secondary">AI</span>
          </div>
        </transition>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="nav-item"
          :class="{ active: isActive(item.to) }"
        >
          <el-icon class="nav-icon"><component :is="item.icon" /></el-icon>
          <transition name="nav-label">
            <span v-if="!appStore.sidebarCollapsed" class="nav-label">{{ t(item.label) }}</span>
          </transition>
          <span v-if="!appStore.sidebarCollapsed && item.badge" class="nav-badge">{{ item.badge }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <button class="collapse-btn" @click="appStore.toggleSidebar">
          <el-icon><component :is="appStore.sidebarCollapsed ? 'Expand' : 'Fold'" /></el-icon>
        </button>
      </div>
    </aside>

    <!-- 主内容区 -->
    <div class="main-wrapper">
      <!-- 顶栏 -->
      <header class="topbar">
        <div class="topbar-left">
          <div class="breadcrumb">
            <span class="breadcrumb-root">Scanner</span>
            <span class="breadcrumb-sep">/</span>
            <span class="breadcrumb-current">{{ currentPageTitle }}</span>
          </div>
        </div>
        <div class="topbar-right">
          <!-- 当前用户 -->
          <span class="topbar-user font-mono">
            <el-icon><UserFilled /></el-icon>
            {{ username }}
          </span>
          <!-- 语言切换 -->
          <button class="topbar-btn lang-btn" @click="toggleLocale">
            <el-icon><Translate /></el-icon>
            <span>{{ appStore.locale === 'zh' ? 'EN' : '中' }}</span>
          </button>
          <!-- 新建扫描快捷按钮 -->
          <router-link to="/new-scan" class="new-scan-btn">
            <el-icon><Plus /></el-icon>
            <span>{{ t('nav.newScan') }}</span>
          </router-link>
          <!-- 退出登录 -->
          <button class="topbar-btn logout-btn" @click="handleLogout" :title="'退出登录'">
            <el-icon><SwitchButton /></el-icon>
          </button>
        </div>
      </header>

      <!-- 页面内容 -->
      <main class="page-content">
        <router-view v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import * as api from '@/api'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const username = computed(() => localStorage.getItem('username') || 'admin')

async function handleLogout() {
  try { await api.logout() } catch {}
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  router.push('/login')
}

const navItems = [
  { to: '/dashboard', label: 'nav.dashboard', icon: 'DataBoard'   },
  { to: '/tasks',     label: 'nav.tasks',     icon: 'List'        },
  { to: '/new-scan',  label: 'nav.newScan',   icon: 'CirclePlus'  },
  { to: '/vulnmap',   label: 'nav.vulnMap',   icon: 'Share'       },
  { to: '/rules',     label: 'nav.rules',     icon: 'Edit'        },
  { to: '/settings',  label: 'nav.settings',  icon: 'Setting'     },
]

const isActive = (path: string) => route.path === path || route.path.startsWith(path + '/')

const currentPageTitle = computed(() => {
  if (route.path.startsWith('/dashboard')) return t('nav.dashboard')
  if (route.path.startsWith('/tasks'))     return t('nav.tasks')
  if (route.path.startsWith('/new-scan'))  return t('nav.newScan')
  if (route.path.startsWith('/vulnmap'))   return t('nav.vulnMap')
  if (route.path.startsWith('/rules'))     return t('nav.rules')
  if (route.path.startsWith('/settings'))  return t('nav.settings')
  return ''
})

function toggleLocale() {
  appStore.toggleLocale()
  locale.value = appStore.locale
}
</script>

<style scoped>
.layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--bg-base);
}

/* 扫描线装饰 */
.scan-line {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--accent-primary), transparent);
  z-index: 1000;
  animation: pulse-glow 3s ease-in-out infinite;
}

/* ===== 侧边栏 ===== */
.sidebar {
  width: var(--sidebar-width);
  min-width: var(--sidebar-width);
  height: 100vh;
  background: var(--bg-surface);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  transition: width var(--transition-normal), min-width var(--transition-normal);
  position: relative;
  z-index: 100;
}

.layout.sidebar-collapsed .sidebar {
  width: 60px;
  min-width: 60px;
}

.sidebar-logo {
  height: var(--header-height);
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-subtle);
  gap: 10px;
  overflow: hidden;
}

.logo-icon {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-text {
  white-space: nowrap;
  overflow: hidden;
}

.logo-primary {
  font-family: var(--font-display);
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 0.5px;
}

.logo-secondary {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
  color: var(--accent-primary);
  margin-left: 4px;
  padding: 1px 5px;
  border: 1px solid var(--accent-primary);
  border-radius: var(--radius-sm);
}

.sidebar-nav {
  flex: 1;
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 13.5px;
  font-weight: 500;
  transition: all var(--transition-fast);
  white-space: nowrap;
  overflow: hidden;
  position: relative;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--accent-glow);
  color: var(--accent-primary);
  border: 1px solid rgba(14, 165, 233, 0.2);
}

.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 60%;
  background: var(--accent-primary);
  border-radius: 0 2px 2px 0;
  box-shadow: 0 0 8px var(--accent-primary);
}

.nav-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.nav-label {
  flex: 1;
}

.nav-badge {
  font-family: var(--font-mono);
  font-size: 10px;
  background: var(--accent-primary);
  color: var(--text-inverse);
  padding: 1px 6px;
  border-radius: 10px;
}

.sidebar-footer {
  padding: 12px 8px;
  border-top: 1px solid var(--border-subtle);
}

.collapse-btn {
  width: 100%;
  padding: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.collapse-btn:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

/* ===== 主内容区 ===== */
.main-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.topbar {
  height: var(--header-height);
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  flex-shrink: 0;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.breadcrumb-root {
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.breadcrumb-sep {
  color: var(--border-default);
}

.breadcrumb-current {
  color: var(--text-primary);
  font-weight: 500;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.topbar-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: none;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
  font-family: var(--font-body);
}

.topbar-btn:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

.new-scan-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: var(--accent-primary);
  border: none;
  border-radius: var(--radius-md);
  color: var(--text-inverse);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  transition: all var(--transition-fast);
  font-family: var(--font-body);
  box-shadow: 0 0 16px rgba(14, 165, 233, 0.3);
}

.new-scan-btn:hover {
  background: #38bdf8;
  box-shadow: 0 0 24px rgba(14, 165, 233, 0.5);
}

.topbar-user {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--text-secondary);
  padding: 0 4px;
}

.logout-btn {
  color: var(--text-muted);
  border-color: var(--border-subtle);
}

.logout-btn:hover {
  border-color: var(--severity-critical) !important;
  color: var(--severity-critical) !important;
}

.page-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

/* ===== 过渡动画 ===== */
.page-enter-active,
.page-leave-active {
  transition: all 0.2s ease;
}
.page-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.page-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.logo-text-enter-active,
.logo-text-leave-active {
  transition: all 0.2s ease;
}
.logo-text-enter-from,
.logo-text-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}
</style>
