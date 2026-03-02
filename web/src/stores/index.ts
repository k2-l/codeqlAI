import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Task, Finding, DashboardStats } from '@/api/types'
import * as api from '@/api'

// ===== Task Store =====
export const useTaskStore = defineStore('task', () => {
  const tasks = ref<Task[]>([])
  const currentTask = ref<Task | null>(null)
  const findings = ref<Finding[]>([])
  const loading = ref(false)

  // 从任务列表计算仪表盘统计（前端聚合，避免额外接口）
  const stats = computed<DashboardStats>(() => {
    const total = tasks.value.length
    const completed = tasks.value.filter(t => t.status === 'completed').length
    const failed = tasks.value.filter(t => t.status === 'failed').length
    const running = tasks.value.filter(t =>
      ['pending','cloning','building','analyzing','ai_reviewing'].includes(t.status)
    ).length

    const allFindings = findings.value
    const high = allFindings.filter(f => f.severity === 'high' || f.severity === 'critical').length
    const audited = allFindings.filter(f => f.audit_status === 'completed').length
    const exploitable = allFindings.filter(f => f.ai_result?.is_exploitable).length

    return {
      total_tasks: total,
      completed_tasks: completed,
      failed_tasks: failed,
      running_tasks: running,
      total_findings: allFindings.length,
      high_findings: high,
      audited_findings: audited,
      exploitable_findings: exploitable,
    }
  })

  async function fetchTasks() {
    // 后端暂无 list 接口，用已知 tasks 更新状态
    // 后续可添加 GET /api/v1/tasks 接口
  }

  function addTask(task: Task) {
    const idx = tasks.value.findIndex(t => t.id === task.id)
    if (idx >= 0) tasks.value[idx] = task
    else tasks.value.unshift(task)
  }

  function removeTask(id: string) {
    tasks.value = tasks.value.filter(t => t.id !== id)
    if (currentTask.value?.id === id) currentTask.value = null
  }

  async function loadTask(id: string) {
    loading.value = true
    try {
      const task = await api.getTask(id)
      currentTask.value = task
      addTask(task)
      return task
    } finally {
      loading.value = false
    }
  }

  async function loadFindings(taskId: string) {
    const res = await api.getFindings(taskId)
    findings.value = res.items
    return res
  }

  return { tasks, currentTask, findings, loading, stats, addTask, removeTask, loadTask, loadFindings, fetchTasks }
})

// ===== App Store =====
export const useAppStore = defineStore('app', () => {
  const locale = ref<'zh' | 'en'>(
    (localStorage.getItem('locale') as 'zh' | 'en') || 'zh'
  )
  const sidebarCollapsed = ref(false)

  function toggleLocale() {
    locale.value = locale.value === 'zh' ? 'en' : 'zh'
    localStorage.setItem('locale', locale.value)
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return { locale, sidebarCollapsed, toggleLocale, toggleSidebar }
})
