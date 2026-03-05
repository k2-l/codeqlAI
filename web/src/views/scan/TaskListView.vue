<template>
  <div class="task-list fade-in-up">
    <div class="page-header">
      <h1 class="page-title font-display">{{ t('task.title') }}</h1>
      <div class="header-actions">
        <el-input
          v-model="searchQuery"
          :placeholder="t('common.search')"
          prefix-icon="Search"
          clearable
          style="width: 220px"
        />
        <button class="icon-btn" @click="refreshAll" :class="{ spinning: refreshing }">
          <el-icon><Refresh /></el-icon>
        </button>
      </div>
    </div>

    <!-- 任务卡片列表 -->
    <div v-if="filteredTasks.length === 0" class="empty-state">
      <div class="empty-inner">
        <div class="empty-hex">
          <el-icon><DocumentRemove /></el-icon>
        </div>
        <p class="empty-title">{{ t('task.noTasks') }}</p>
        <router-link to="/new-scan" class="btn-primary">
          <el-icon><Plus /></el-icon>
          {{ t('nav.newScan') }}
        </router-link>
      </div>
    </div>

    <div v-else class="task-grid">
      <div
        v-for="task in filteredTasks"
        :key="task.id"
        class="task-card"
        :class="`card-${task.status}`"
      >
        <!-- 卡片顶部 -->
        <div class="card-top">
          <div class="card-status-bar" :class="`bar-${task.status}`" />
          <div class="card-header">
            <div class="card-name" :title="task.display_name">
              {{ task.display_name || task.id }}
            </div>
            <div class="card-badges">
              <span class="lang-badge font-mono">{{ task.language }}</span>
              <span class="source-badge font-mono" v-if="task.project">
                {{ task.project.source_type === 'git' ? 'GIT' : 'LOCAL' }}
              </span>
            </div>
          </div>
        </div>

        <!-- 状态进度 -->
        <div class="card-progress">
          <div class="progress-steps">
            <div
              v-for="step in progressSteps"
              :key="step.key"
              class="progress-step"
              :class="getStepClass(task.status, step.key)"
            >
              <div class="step-dot">
                <el-icon v-if="isStepDone(task.status, step.key)"><Check /></el-icon>
                <span v-else-if="isStepActive(task.status, step.key)" class="dot-pulse" />
              </div>
              <span class="step-label">{{ step.label }}</span>
            </div>
          </div>
          <div class="status-chip" :class="`chip-${task.status}`">
            {{ t(`status.${task.status}`) }}
          </div>
        </div>

        <!-- 元信息 -->
        <div class="card-meta">
          <span class="meta-item font-mono">
            <el-icon><Clock /></el-icon>
            {{ formatTime(task.created_at) }}
          </span>
          <span v-if="task.finished_at" class="meta-item font-mono">
            <el-icon><Timer /></el-icon>
            {{ getDuration(task.started_at, task.finished_at) }}
          </span>
          <span v-if="task.project" class="meta-item project-name" :title="task.project.name">
            {{ task.project.name }}
          </span>
        </div>

        <!-- 错误信息 -->
        <div v-if="task.error_log && task.status === 'failed'" class="card-error font-mono">
          {{ task.error_log.slice(0, 120) }}{{ task.error_log.length > 120 ? '...' : '' }}
        </div>

        <!-- 操作按钮 -->
        <div class="card-actions">
          <router-link
            v-if="task.status === 'completed'"
            :to="`/tasks/${task.id}`"
            class="btn-primary btn-sm"
          >
            <el-icon><View /></el-icon>
            {{ t('task.viewResults') }}
          </router-link>
          <button
            v-if="isRunning(task.status)"
            class="btn-secondary btn-sm"
            @click="refreshTask(task.id)"
          >
            <el-icon><Refresh /></el-icon>
            {{ t('common.refresh') }}
          </button>
          <button
            class="btn-danger btn-sm"
            @click="handleDelete(task)"
          >
            <el-icon><Delete /></el-icon>
            {{ t('task.delete') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox, ElMessage } from 'element-plus'
import { useTaskStore } from '@/stores'
import * as api from '@/api'
import dayjs from 'dayjs'
import type { Task, TaskStatus } from '@/api/types'

const { t } = useI18n()
const taskStore = useTaskStore()

const searchQuery = ref('')
const refreshing = ref(false)

// 进度步骤
const progressSteps = [
  { key: 'cloning',   label: 'Clone' },
  { key: 'building',  label: 'Build' },
  { key: 'analyzing', label: 'Analyze' },
  { key: 'completed', label: 'Done' },
]

const statusOrder: TaskStatus[] = ['pending','cloning','building','analyzing','ai_reviewing','completed','failed']

const isRunning = (status: TaskStatus) =>
  ['pending','cloning','building','analyzing','ai_reviewing'].includes(status)

function getStepIndex(status: TaskStatus) {
  return statusOrder.indexOf(status)
}

function isStepDone(taskStatus: TaskStatus, stepKey: string) {
  if (taskStatus === 'failed') return false
  const stepIdx = progressSteps.findIndex(s => s.key === stepKey)
  const currentIdx = getStepIndex(taskStatus)
  const stepStatusIdx = getStepIndex(stepKey as TaskStatus)
  return currentIdx > stepStatusIdx
}

function isStepActive(taskStatus: TaskStatus, stepKey: string) {
  return taskStatus === stepKey
}

function getStepClass(taskStatus: TaskStatus, stepKey: string) {
  if (taskStatus === 'failed') return 'step-idle'
  if (isStepDone(taskStatus, stepKey)) return 'step-done'
  if (isStepActive(taskStatus, stepKey)) return 'step-active'
  return 'step-idle'
}

const filteredTasks = computed(() => {
  if (!searchQuery.value) return taskStore.tasks
  const q = searchQuery.value.toLowerCase()
  return taskStore.tasks.filter(t =>
    t.display_name?.toLowerCase().includes(q) ||
    t.project?.name?.toLowerCase().includes(q) ||
    t.language?.includes(q)
  )
})

async function refreshTask(id: string) {
  const task = await api.getTask(id)
  taskStore.addTask(task)
}

async function refreshAll() {
  refreshing.value = true
  const running = taskStore.tasks.filter(t => isRunning(t.status))
  await Promise.all(running.map(t => refreshTask(t.id)))
  refreshing.value = false
}

async function handleDelete(task: Task) {
  try {
    await ElMessageBox.confirm(t('task.confirmDelete'), t('task.delete'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
      customClass: 'dark-msgbox',
      appendTo: 'body',
    })
  } catch {
    return // 用户点了取消，正常退出
  }

  try {
    await api.deleteTask(task.id)
    taskStore.removeTask(task.id)
    ElMessage.success(t('task.deleteSuccess'))
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || t('task.deleteFailed') || '删除失败')
  }
}

const formatTime = (s: string) => dayjs(s).format('YYYY-MM-DD HH:mm')
const getDuration = (start?: string, end?: string) => {
  if (!start || !end) return ''
  const sec = dayjs(end).diff(dayjs(start), 'second')
  if (sec < 60) return `${sec}s`
  return `${Math.floor(sec / 60)}m ${sec % 60}s`
}

// 自动轮询进行中的任务
let pollTimer: ReturnType<typeof setInterval>
onMounted(async () => {
  // 每次进入页面都从后端拉取最新任务列表，保证后端重启或数据变化后数据不丢失
  await taskStore.fetchTasks()

  pollTimer = setInterval(() => {
    const running = taskStore.tasks.filter(t => isRunning(t.status))
    running.forEach(t => refreshTask(t.id))
  }, 5000)
})
onUnmounted(() => clearInterval(pollTimer))
</script>

<style scoped>
.task-list { max-width: 1200px; }

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.icon-btn {
  width: 36px;
  height: 36px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--transition-fast);
}

.icon-btn:hover { border-color: var(--accent-primary); color: var(--accent-primary); }
.icon-btn.spinning .el-icon { animation: spin 1s linear infinite; }

@keyframes spin { to { transform: rotate(360deg); } }

/* 卡片网格 */
.task-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
}

.task-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all var(--transition-normal);
}

.task-card:hover {
  border-color: var(--border-default);
  box-shadow: 0 4px 20px rgba(0,0,0,0.3);
}

.card-failed { border-color: rgba(244,63,94,0.25); }
.card-completed { border-color: rgba(34,197,94,0.2); }

.card-status-bar {
  height: 3px;
  width: 100%;
}

.bar-pending    { background: var(--status-pending); }
.bar-cloning,
.bar-building,
.bar-analyzing  { background: linear-gradient(90deg, var(--accent-primary) 0%, transparent 100%); animation: scan-bar 2s ease infinite; }
.bar-completed  { background: var(--status-completed); }
.bar-failed     { background: var(--status-failed); }

@keyframes scan-bar {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}

.card-header {
  padding: 14px 16px 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-badges {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.lang-badge, .source-badge {
  font-size: 10px;
  padding: 2px 7px;
  border-radius: var(--radius-sm);
  font-weight: 600;
}

.lang-badge {
  background: var(--accent-glow);
  color: var(--accent-primary);
  border: 1px solid rgba(14,165,233,0.2);
}

.source-badge {
  background: var(--bg-elevated);
  color: var(--text-muted);
  border: 1px solid var(--border-subtle);
}

/* 进度步骤 */
.card-progress {
  padding: 12px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.progress-steps {
  display: flex;
  align-items: center;
  gap: 0;
  flex: 1;
}

.progress-step {
  display: flex;
  align-items: center;
  gap: 0;
  flex: 1;
}

.progress-step:not(:last-child)::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border-subtle);
  margin: 0 4px;
}

.step-done::after   { background: var(--accent-primary) !important; }
.step-active::after { background: var(--accent-primary) !important; }

.step-dot {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1.5px solid var(--border-default);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  flex-shrink: 0;
  position: relative;
}

.step-done .step-dot {
  border-color: var(--accent-primary);
  background: var(--accent-primary);
  color: var(--text-inverse);
}

.step-active .step-dot {
  border-color: var(--accent-primary);
  background: var(--accent-glow);
  color: var(--accent-primary);
}

.dot-pulse {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-primary);
  animation: pulse-glow 1s ease-in-out infinite;
}

.step-label {
  display: none;
}

.status-chip {
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: var(--radius-sm);
  letter-spacing: 0.5px;
  white-space: nowrap;
}

.chip-pending    { background: rgba(107,114,128,0.15); color: var(--status-pending); }
.chip-cloning,
.chip-building,
.chip-analyzing  { background: var(--accent-glow); color: var(--accent-primary); }
.chip-completed  { background: rgba(34,197,94,0.1); color: var(--status-completed); }
.chip-failed     { background: rgba(244,63,94,0.1); color: var(--status-failed); }

/* 元信息 */
.card-meta {
  padding: 0 16px 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-muted);
}

.project-name {
  font-family: var(--font-body);
  font-size: 11px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 120px;
}

/* 错误 */
.card-error {
  margin: 0 16px 12px;
  padding: 8px 10px;
  background: rgba(244,63,94,0.08);
  border: 1px solid rgba(244,63,94,0.2);
  border-radius: var(--radius-sm);
  font-size: 11px;
  color: #f87171;
  line-height: 1.5;
  word-break: break-all;
}

/* 操作 */
.card-actions {
  padding: 10px 16px 14px;
  display: flex;
  gap: 8px;
  border-top: 1px solid var(--border-subtle);
}

.btn-primary, .btn-secondary, .btn-danger {
  display: flex;
  align-items: center;
  gap: 5px;
  border-radius: var(--radius-md);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
  border: 1px solid transparent;
  transition: all var(--transition-fast);
  font-family: var(--font-body);
}

.btn-sm { padding: 5px 12px; }

.btn-primary {
  background: var(--accent-primary);
  color: var(--text-inverse);
  box-shadow: 0 0 12px rgba(14,165,233,0.3);
}

.btn-primary:hover {
  background: #38bdf8;
  box-shadow: 0 0 20px rgba(14,165,233,0.5);
}

.btn-secondary {
  background: var(--bg-elevated);
  color: var(--text-secondary);
  border-color: var(--border-default);
}

.btn-secondary:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

.btn-danger {
  background: rgba(244,63,94,0.08);
  color: var(--severity-critical);
  border-color: rgba(244,63,94,0.2);
}

.btn-danger:hover {
  background: rgba(244,63,94,0.15);
  border-color: var(--severity-critical);
}

/* 空状态 */
.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 300px;
}

.empty-inner {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.empty-hex {
  width: 80px;
  height: 80px;
  border: 2px solid var(--border-default);
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  color: var(--text-muted);
}

.empty-title {
  font-size: 15px;
  color: var(--text-secondary);
}
</style>